package com.copyapp.companion

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.os.Bundle
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import com.google.zxing.integration.android.IntentIntegrator

class MainActivity : Activity() {
    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        val layout = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(32, 40, 32, 32) }
        layout.addView(TextView(this).apply { text = "CopyApp\n\nPair once: open a fresh QR code on the Windows PC, then scan it here. Compare the full certificate fingerprint before confirming.\n\nAfter pairing, edit Android Quick Settings and add ‘CopyApp: Send to PC’. Copy text, then tap that tile. Images: use the existing CopyApp browser Share flow.\n\nChanged PC address? Scan a fresh QR code again." })
        layout.addView(Button(this).apply { text = "Scan PC QR code"; setOnClickListener { IntentIntegrator(this@MainActivity).setDesiredBarcodeFormats(IntentIntegrator.QR_CODE).setPrompt("Scan the QR displayed on your Windows PC").setBeepEnabled(false).initiateScan() } })
        layout.addView(Button(this).apply { text = "Send clipboard to PC"; setOnClickListener { startActivity(Intent(this@MainActivity, SendClipboardActivity::class.java)) } })
        layout.addView(Button(this).apply { text = "Forget this PC on phone"; setOnClickListener { Connection(this@MainActivity).clear(); AlertDialog.Builder(this@MainActivity).setMessage("Removed from this phone. Use Unpair on Windows to revoke PC credentials.").setPositiveButton("OK", null).show() } })
        setContentView(layout)
    }
    @Deprecated("Scanner Activity API")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        val result = IntentIntegrator.parseActivityResult(requestCode, resultCode, data)
        if (result == null) { super.onActivityResult(requestCode, resultCode, data); return }
        val raw = result.contents ?: return
        try {
            val connection = Connection(this)
            val pairing = connection.parse(raw)
            AlertDialog.Builder(this).setTitle("Verify this PC")
                .setMessage("Compare this entire SHA-256 certificate fingerprint with the full fingerprint shown in the PC pairing window:\n\n${pairing.fingerprint}\n\nOnly confirm if they match.")
                .setNegativeButton("Cancel", null)
                .setPositiveButton("Fingerprints match") { _, _ ->
                    Thread {
                        val message = try { connection.pair(pairing); "Paired. Add ‘CopyApp: Send to PC’ in Quick Settings, or use the Send clipboard button." } catch (_: Exception) { "Pairing failed. Check Wi-Fi and the PC address, then open and scan a fresh QR code." }
                        runOnUiThread { if (!isFinishing) AlertDialog.Builder(this).setMessage(message).setPositiveButton("OK", null).show() }
                    }.start()
                }.show()
        } catch (e: Exception) {
            AlertDialog.Builder(this).setMessage(e.message ?: "Invalid QR code").setPositiveButton("OK", null).show()
        }
    }
}
