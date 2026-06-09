package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.ui.Messages

class SearchAction : PithAction("Search...", "Search the project for a query") {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val basePath = project.basePath ?: return

        val selected = e.getData(CommonDataKeys.EDITOR)?.selectionModel?.selectedText ?: ""
        val query = Messages.showInputDialog(project, "Search query:", "pith search", null, selected, null)
            ?: return

        runPith(e, listOf("search", query, basePath, "-r"))
    }
}
