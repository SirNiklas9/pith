package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys

class ReadAction : PithAction("Read File", "Purpose overview of the current file") {
    override fun actionPerformed(e: AnActionEvent) {
        val file = e.getData(CommonDataKeys.VIRTUAL_FILE)?.path ?: return
        runPith(e, listOf("read", file, "--grep"))
    }
}
