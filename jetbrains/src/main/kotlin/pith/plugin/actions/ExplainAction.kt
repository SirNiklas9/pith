package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.ui.Messages
import pith.plugin.PithSettings

class ExplainAction : PithAction("Explain", "Deep explanation of the selected declaration") {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val file    = e.getData(CommonDataKeys.VIRTUAL_FILE)?.path ?: return
        val editor  = e.getData(CommonDataKeys.EDITOR) ?: return
        val agent   = PithSettings.getInstance().state.agentCommand

        val selected = editor.selectionModel.selectedText?.trim() ?: ""
        val name = if (selected.isNotEmpty()) selected else {
            Messages.showInputDialog(project, "Declaration name:", "pith explain", null) ?: return
        }

        runPith(e, listOf("explain", file, name, "--agent", agent))
    }
}
