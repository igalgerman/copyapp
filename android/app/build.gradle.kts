plugins { id("com.android.application"); id("org.jetbrains.kotlin.android") }

val releaseStore = providers.environmentVariable("COPYAPP_ANDROID_KEYSTORE_PATH").orNull
val releaseStorePassword = providers.environmentVariable("COPYAPP_ANDROID_KEYSTORE_PASSWORD").orNull
val releaseKeyAlias = providers.environmentVariable("COPYAPP_ANDROID_KEY_ALIAS").orNull
val releaseKeyPassword = providers.environmentVariable("COPYAPP_ANDROID_KEY_PASSWORD").orNull
val releaseSigningReady = listOf(releaseStore, releaseStorePassword, releaseKeyAlias, releaseKeyPassword).all { !it.isNullOrBlank() }

android {
    namespace = "com.copyapp.companion"
    compileSdk = 35
    defaultConfig {
        applicationId = "com.copyapp.companion"
        minSdk = 26
        targetSdk = 35
        versionCode = providers.environmentVariable("COPYAPP_ANDROID_VERSION_CODE").orElse("1").get().toInt()
        versionName = providers.environmentVariable("COPYAPP_VERSION").orElse("dev").get()
    }
    signingConfigs {
        if (releaseSigningReady) {
            create("ownerRelease") {
                storeFile = file(releaseStore!!)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }
    buildTypes {
        getByName("release") {
            if (releaseSigningReady) signingConfig = signingConfigs.getByName("ownerRelease")
        }
    }
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }
    kotlinOptions { jvmTarget = "17" }
}
// Never produce a release APK/bundle that appears ready to install without the
// owner's stable key. Debug builds and lint remain usable without any secrets.
tasks.configureEach {
    if (name == "preReleaseBuild") {
        doFirst { check(releaseSigningReady) { "Release signing is not configured. See docs/ANDROID.md." } }
    }
}
dependencies { implementation("com.journeyapps:zxing-android-embedded:4.3.0") }
