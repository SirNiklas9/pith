package pith.plugin

import com.intellij.execution.filters.TextConsoleBuilderFactory
import com.intellij.execution.ui.ConsoleView
import com.intellij.execution.ui.ConsoleViewContentType
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Key
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.content.ContentFactory

class PithToolWindowFactory : ToolWindowFactory {

    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val console = TextConsoleBuilderFactory.getInstance()
            .createBuilder(project)
            .console
        val content = ContentFactory.getInstance()
            .createContent(console.component, "", false)
        toolWindow.contentManager.addContent(content)
        project.putUserData(CONSOLE_KEY, console)
    }

    companion object {
        private val CONSOLE_KEY = Key.create<ConsoleView>("pith.console")

        fun print(project: Project, text: String, error: Boolean = false) {
            val console = project.getUserData(CONSOLE_KEY) ?: return
            val type = if (error) ConsoleViewContentType.ERROR_OUTPUT
                       else ConsoleViewContentType.NORMAL_OUTPUT
            console.print(text, type)
        }

        fun clear(project: Project) {
            project.getUserData(CONSOLE_KEY)?.clear()
        }
    }
}
