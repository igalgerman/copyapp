package com.copyapp.companion

import android.content.Context
import android.util.Base64
import org.json.JSONObject
import java.net.URL
import java.security.KeyStore
import java.security.MessageDigest
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory

class Connection(private val context: Context) {
    private val prefs = context.getSharedPreferences("pairing", Context.MODE_PRIVATE)
    fun paired() = prefs.contains("credential")
    fun clear() { prefs.edit().clear().commit() }
    data class Pairing(val origin: String, val token: String, val ca: ByteArray, val fingerprint: String)
    fun parse(raw: String): Pairing {
        val uri = android.net.Uri.parse(raw)
        require(uri.scheme == "https" && uri.path == "/pair" && uri.userInfo == null) { "Scan the current PC pairing QR code" }
        val host = uri.host ?: error("Missing PC address")
        require(host.isNotBlank())
        val token = uri.getQueryParameter("t") ?: error("Missing pairing token")
        require(token.matches(Regex("[0-9a-f]{64}"))) { "Invalid token" }
        val fragment = android.net.Uri.parse("https://local/?" + (uri.fragment ?: ""))
        val ca = Base64.decode(fragment.getQueryParameter("ca") ?: error("Update CopyApp on Windows; this QR does not support the companion"), Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)
        require(ca.size <= 4096) { "Invalid certificate" }
        val cert = certificate(ca)
        require(cert.basicConstraints >= 0) { "Expected PC CA certificate" }
        cert.checkValidity()
        val fp = MessageDigest.getInstance("SHA-256").digest(cert.encoded).joinToString(":") { "%02X".format(it.toInt() and 255) }
        return Pairing("https://" + (if (host.contains(':')) "[$host]" else host) + (if (uri.port > 0) ":${uri.port}" else ""), token, ca, fp)
    }
    private fun certificate(ca: ByteArray) = CertificateFactory.getInstance("X.509").generateCertificate(ca.inputStream()) as X509Certificate
    private fun post(origin: String, ca: ByteArray, path: String, payload: JSONObject, credential: String? = null): JSONObject {
        val store = KeyStore.getInstance(KeyStore.getDefaultType()).apply { load(null); setCertificateEntry("copyapp", certificate(ca)) }
        val managers = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm()).apply { init(store) }
        val tls = SSLContext.getInstance("TLS").apply { init(null, managers.trustManagers, null) }
        val conn = URL(origin + path).openConnection() as HttpsURLConnection
        try {
            conn.sslSocketFactory = tls.socketFactory
            // Keep the platform hostname verifier: a pin never bypasses host verification.
            conn.instanceFollowRedirects = false
            conn.connectTimeout = 5000; conn.readTimeout = 5000
            conn.requestMethod = "POST"; conn.doOutput = true
            conn.setRequestProperty("Content-Type", "application/json")
            if (credential != null) conn.setRequestProperty("Authorization", "Bearer $credential")
            conn.outputStream.use { it.write(payload.toString().toByteArray(Charsets.UTF_8)) }
            val status = conn.responseCode
            if (status == 401) error("Pairing expired or revoked. Scan a new PC QR code.")
            check(status in 200..299) { "PC refused the send (HTTP $status). Try again." }
            val body = conn.inputStream.bufferedReader().use { it.readText() }
            return if (body.isBlank()) JSONObject() else JSONObject(body)
        } finally { conn.disconnect() }
    }
    fun pair(pairing: Pairing) {
        val response = post(pairing.origin, pairing.ca, "/companion/pair", JSONObject().put("token", pairing.token))
        val credential = response.getString("credential")
        check(credential.matches(Regex("[0-9a-f]{64}"))) { "Invalid PC response" }
        check(prefs.edit().putString("origin", pairing.origin).putString("ca", Base64.encodeToString(pairing.ca, Base64.NO_WRAP)).putString("credential", credential).commit()) { "Cannot save pairing" }
    }
    fun send(text: String) {
        check(paired()) { "Open CopyApp and pair with your PC first" }
        val ca = Base64.decode(prefs.getString("ca", ""), Base64.DEFAULT)
        val result = post(prefs.getString("origin", "")!!, ca, "/companion/send", JSONObject().put("text", text).put("mode", "clipboard"), prefs.getString("credential", null))
        check(result.optString("where") == "clipboard") { "PC did not confirm clipboard delivery" }
    }
}
