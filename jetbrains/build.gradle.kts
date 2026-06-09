plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "1.9.24"
    id("org.jetbrains.intellij") version "1.17.3"
}

group = "io.pith"
version = "0.1.0"

repositories {
    mavenCentral()
}

intellij {
    version.set("2024.1")
    type.set("IC")
}

kotlin {
    jvmToolchain(17)
}

tasks {
    buildSearchableOptions { enabled = false }
    patchPluginXml {
        sinceBuild.set("241")
        untilBuild.set("")
    }
}
