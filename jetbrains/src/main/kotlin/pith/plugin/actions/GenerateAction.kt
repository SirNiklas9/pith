package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VirtualFileManager
import pith.plugin.PithSettings

class GenerateAction : PithAction("Generate File...", "Generate a new file from a prompt") {
    override fun actionPerformed(e: AnActionEvent) {
        val project  = e.project ?: return
        val basePath = project.basePath ?: return
        val backend  = PithSettings.getInstance().backendArgs()

        val relPath = Messages.showInputDialog(
            project, "New file path (relative to project root):", "pith generate", null
        ) ?: return

        val prompt = Messages.showInputDialog(
            project, "What to generate:", "pith generate — $relPath", null
        ) ?: return

        val fullPath = "$basePath/$relPath"

        runPith(e, listOf("generate", fullPath, "--prompt", prompt, "--apply") + backend) {
            // Refresh VFS then open the new file — the Runnable fires after refresh completes
            VirtualFileManager.getInstance().asyncRefresh {
                val newFile = LocalFileSystem.getInstance().refreshAndFindFileByPath(fullPath)
                if (newFile != null) {
                    FileEditorManager.getInstance(project).openFile(newFile, true)
                }
            }
        }
    }
}
