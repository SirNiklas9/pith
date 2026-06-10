plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "1.9.24"
    id("org.jetbrains.intellij") version "1.17.3"
}

group = "io.pith"
version = "0.4.0"

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

// ── bundled pith binaries ────────────────────────────────────────────────────
// The plugin ships the pith CLI inside its own jar (resources/bin/) so a fresh
// install needs no PATH setup. Dev builds bundle only the host platform;
// release builds pass -PbundlePlatforms=all. The Go toolchain must be on PATH.

val allPlatforms = listOf(
    "windows-amd64", "windows-arm64",
    "linux-amd64", "linux-arm64",
    "darwin-amd64", "darwin-arm64",
)

fun hostPlatform(): String {
    val os = System.getProperty("os.name").lowercase()
    val goos = when {
        os.contains("win") -> "windows"
        os.contains("mac") || os.contains("darwin") -> "darwin"
        else -> "linux"
    }
    val arch = System.getProperty("os.arch").lowercase()
    val goarch = if (arch.contains("aarch64") || arch.contains("arm64")) "arm64" else "amd64"
    return "$goos-$goarch"
}

val bundlePlatforms: List<String> = when (val p = (findProperty("bundlePlatforms") as String?) ?: "host") {
    "all" -> allPlatforms
    "host" -> listOf(hostPlatform())
    "none" -> emptyList()
    else -> p.split(",").map { it.trim() }
}

val pithBinDir = layout.buildDirectory.dir("pith-bin")

val buildPithBinaries by tasks.registering {
    outputs.upToDateWhen { false } // go's own build cache makes this cheap
    doLast {
        val outDir = pithBinDir.get().asFile
        outDir.mkdirs()
        val repoRoot = rootDir.parentFile // jetbrains/ lives inside the pith repo
        for (platform in bundlePlatforms) {
            val (goos, goarch) = platform.split("-")
            val ext = if (goos == "windows") ".exe" else ""
            exec {
                workingDir = repoRoot
                environment("GOOS", goos)
                environment("GOARCH", goarch)
                commandLine(
                    "go", "build", "-ldflags", "-s -w",
                    "-o", File(outDir, "pith-$platform$ext").absolutePath,
                    "./cmd/pith",
                )
            }
        }
    }
}

tasks {
    processResources {
        dependsOn(buildPithBinaries)
        from(pithBinDir) { into("bin") }
    }
    buildSearchableOptions { enabled = false }
    instrumentCode         { enabled = false }
    patchPluginXml {
        sinceBuild.set("241")
        untilBuild.set("")
    }
}
