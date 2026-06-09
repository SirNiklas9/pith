package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys

class ReadPackageAction : PithAction("Read Package", "Purpose overview of the whole package or folder") {
    override fun actionPerformed(e: AnActionEvent) {
        val dir = e.getData(CommonDataKeys.VIRTUAL_FILE)?.parent?.path ?: return
        runPith(e, listOf("read", dir))
    }
}
