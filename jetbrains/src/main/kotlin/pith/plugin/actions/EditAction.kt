package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.vfs.VirtualFileManager
import pith.plugin.PithSettings

class EditAction : PithAction("Edit Selection...", "AI edit of the selected region") {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val file    = e.getData(CommonDataKeys.VIRTUAL_FILE)?.path ?: return
        val editor  = e.getData(CommonDataKeys.EDITOR) ?: return
        val agent   = PithSettings.getInstance().state.agentCommand

        val doc   = editor.document
        val sel   = editor.selectionModel
        val start = doc.getLineNumber(sel.selectionStart) + 1
        val end   = doc.getLineNumber(sel.selectionEnd)   + 1

        val prompt = Messages.showInputDialog(
            project, "Edit instruction (lines $start–$end):", "pith edit", null
        ) ?: return

        runPith(e, listOf("edit", file, "--range", "$start:$end", "--prompt", prompt, "--agent", agent, "--apply"))

        e.getData(CommonDataKeys.VIRTUAL_FILE)?.refresh(true, false)
        VirtualFileManager.getInstance().asyncRefresh(null)
    }
}
