plugins { id("com.android.application"); id("org.jetbrains.kotlin.android") }
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
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }
    kotlinOptions { jvmTarget = "17" }
}
dependencies { implementation("com.journeyapps:zxing-android-embedded:4.3.0") }
