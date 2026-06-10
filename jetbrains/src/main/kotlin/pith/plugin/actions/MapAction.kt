package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import pith.plugin.PithSettings

class MapAction : PithAction("Map Project", "One row per package: size plus a cached one-line purpose") {
    override fun actionPerformed(e: AnActionEvent) {
        val base = e.project?.basePath ?: return
        // Purposes are cached by content hash, so this is cheap after the first
        // run. In "pith config" mode the CLI gets --ai (its explicit opt-in to
        // the stored backend); otherwise the configured backend flags opt in.
        val backend = PithSettings.getInstance().backendArgs()
        val args = listOf("map", base) + if (backend.isEmpty()) listOf("--ai") else backend
        runPith(e, args)
    }
}
