package com.copyapp.companion

import android.app.Activity
import android.app.AlertDialog
import android.content.ClipboardManager
import android.os.Bundle
import android.widget.TextView
import android.widget.Toast
import java.util.concurrent.atomic.AtomicBoolean

// Clipboard access occurs only after this visible Activity has window focus.
class SendClipboardActivity : Activity() {
    private val started = AtomicBoolean(false)
    // A second Activity cannot start an overlapping send even if the original
    // was destroyed while its bounded HTTP request is still running.
    companion object { private val inFlight = AtomicBoolean(false) }
    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        setContentView(TextView(this).apply { text = "Sending clipboard to PC…"; textSize = 22f; setPadding(32, 64, 32, 32) })
        if (state?.getBoolean("attempted") == true) {
            started.set(true)
            // A lost response may already have changed the PC clipboard. Never
            // silently resend when Android recreates this Activity/process.
            window.decorView.post { fail("The previous send was interrupted. Check the PC clipboard before choosing Retry.") }
        }
    }
    override fun onSaveInstanceState(outState: Bundle) {
        outState.putBoolean("attempted", started.get())
        super.onSaveInstanceState(outState)
    }
    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (!hasFocus || !started.compareAndSet(false, true)) return
        window.decorView.post {
            if (!hasWindowFocus()) { started.set(false); return@post }
            try {
                val clip = (getSystemService(CLIPBOARD_SERVICE) as ClipboardManager).primaryClip
                val text = if (clip != null && clip.itemCount > 0) clip.getItemAt(0).text?.toString() else null
                if (text.isNullOrEmpty()) { fail("Clipboard is empty, expired, or contains an image. Copy text again. For images, use Share to the CopyApp browser app."); return@post }
                if (text.toByteArray(Charsets.UTF_8).size > 1024 * 1024) { fail("Text is too large. Copy a smaller selection."); return@post }
                if (!inFlight.compareAndSet(false, true)) {
                    fail("A previous send is still finishing. Wait a few seconds, check the PC clipboard, then Retry if needed.")
                    return@post
                }
                val connection = Connection(applicationContext)
                Thread {
                    val success = try { connection.send(text); true } catch (_: Exception) { false }
                    inFlight.set(false)
                    runOnUiThread {
                        if (!isFinishing && !isDestroyed) {
                            if (success) { Toast.makeText(this, "Sent to PC", Toast.LENGTH_SHORT).show(); finish() }
                            else fail("Send failed. Check that Windows CopyApp is running on the same Wi-Fi. If the PC address changed or pairing was revoked, open CopyApp and scan a new QR code.")
                        }
                    }
                }.start()
            } catch (_: SecurityException) { fail("Android refused clipboard access. Unlock, copy the text again and retry.") }
        }
    }
    private fun fail(message: String) {
        if (isFinishing || isDestroyed) return
        AlertDialog.Builder(this).setTitle("CopyApp could not send").setMessage(message)
            .setPositiveButton("Retry") { _, _ -> started.set(false); if (hasWindowFocus()) onWindowFocusChanged(true) }
            .setNegativeButton("Close") { _, _ -> finish() }.setOnCancelListener { finish() }.show()
    }
}
