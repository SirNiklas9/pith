package pith.plugin

import com.intellij.ide.plugins.PluginManagerCore
import com.intellij.openapi.application.PathManager
import com.intellij.openapi.extensions.PluginId
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption

/**
 * Resolves the pith executable the plugin runs. Order:
 *  1. an explicit path in Settings (anything other than blank or the bare
 *     legacy default "pith"),
 *  2. the binary bundled inside the plugin jar, extracted once per plugin
 *     version into the IDE system dir,
 *  3. "pith" on PATH.
 */
object PithBinary {

    @Volatile
    private var extractedPath: String? = null

    fun resolve(): String {
        val configured = PithSettings.getInstance().state.pithBinary.trim()
        if (configured.isNotEmpty() && configured != "pith") return configured
        extractedPath?.let { return it }
        synchronized(this) {
            extractedPath?.let { return it }
            val extracted = extractBundled()
            if (extracted != null) {
                extractedPath = extracted
                return extracted
            }
        }
        return "pith"
    }

    /** What the bundled fallback resolves to, for the settings UI. */
    fun bundledHint(): String {
        val (goos, goarch) = hostPlatform()
        val res = resourceName(goos, goarch)
        return if (javaClass.getResource(res) != null)
            "bundled binary ($goos-$goarch, plugin ${pluginVersion()})"
        else
            "no $goos-$goarch binary bundled in this build — falls back to PATH"
    }

    private fun hostPlatform(): Pair<String, String> {
        val os = System.getProperty("os.name").lowercase()
        val goos = when {
            os.contains("win") -> "windows"
            os.contains("mac") || os.contains("darwin") -> "darwin"
            else -> "linux"
        }
        val arch = System.getProperty("os.arch").lowercase()
        val goarch = if (arch.contains("aarch64") || arch.contains("arm64")) "arm64" else "amd64"
        return goos to goarch
    }

    private fun resourceName(goos: String, goarch: String): String =
        "/bin/pith-$goos-$goarch" + if (goos == "windows") ".exe" else ""

    private fun pluginVersion(): String =
        PluginManagerCore.getPlugin(PluginId.getId("io.pith"))?.version ?: "dev"

    private fun extractBundled(): String? {
        val (goos, goarch) = hostPlatform()
        val ext = if (goos == "windows") ".exe" else ""
        val target = Path.of(PathManager.getSystemPath(), "pith", pluginVersion(), "pith$ext")

        val stream = javaClass.getResourceAsStream(resourceName(goos, goarch))
            ?: return if (Files.isRegularFile(target)) target.toString() else null
        val bytes = stream.use { it.readBytes() }

        // Same version number does not mean same binary — a refreshed plugin
        // zip can carry a newer build under the same version. Re-extract
        // whenever the bundled bytes differ in size from what's on disk.
        if (Files.isRegularFile(target) && Files.size(target) == bytes.size.toLong()) {
            return target.toString()
        }

        Files.createDirectories(target.parent)
        val tmp = Files.createTempFile(target.parent, "pith", ".tmp")
        Files.write(tmp, bytes)
        try {
            Files.move(tmp, target, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
        } catch (e: Exception) {
            // lost a race with another extraction; whatever landed there wins
            Files.deleteIfExists(tmp)
            if (!Files.isRegularFile(target)) return null
        }
        if (goos != "windows") File(target.toString()).setExecutable(true)
        return target.toString()
    }
}
