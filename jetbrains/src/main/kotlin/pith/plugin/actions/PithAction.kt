package pith.plugin.actions

import com.intellij.openapi.actionSystem.ActionUpdateThread
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.wm.ToolWindowManager
import pith.plugin.PithRunner
import pith.plugin.PithToolWindowFactory

abstract class PithAction(text: String, description: String) : AnAction(text, description, null) {

    override fun getActionUpdateThread() = ActionUpdateThread.BGT

    override fun update(e: AnActionEvent) {
        e.presentation.isEnabled = e.project != null
    }

    protected fun runPith(e: AnActionEvent, args: List<String>) {
        val project = e.project ?: return
        val workDir = project.basePath ?: return

        ToolWindowManager.getInstance(project).getToolWindow("pith")?.show()
        PithToolWindowFactory.clear(project)
        PithToolWindowFactory.print(project, "pith ${args.joinToString(" ")}\n\n")

        PithRunner.run(
            args    = args,
            workDir = workDir,
            onOutput = { text -> PithToolWindowFactory.print(project, text) },
            onDone   = {}
        )
    }
}
