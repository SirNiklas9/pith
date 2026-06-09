package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import pith.plugin.PithSettings

class SummaryAction : PithAction("Summary", "AI summary of the current file") {
    override fun actionPerformed(e: AnActionEvent) {
        val file  = e.getData(CommonDataKeys.VIRTUAL_FILE)?.path ?: return
        val agent = PithSettings.getInstance().state.agentCommand
        runPith(e, listOf("summary", file, "--agent", agent))
    }
}
