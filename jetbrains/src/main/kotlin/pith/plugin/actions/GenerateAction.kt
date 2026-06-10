package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VirtualFileManager
import pith.plugin.PithPromptDialog
import pith.plugin.PithSettings

class GenerateAction : PithAction("Generate File...", "Generate a new file from a prompt") {
    override fun actionPerformed(e: AnActionEvent) {
        val project  = e.project ?: return
        val basePath = project.basePath ?: return
        val backend  = PithSettings.getInstance().backendArgs()

        val dialog = PithPromptDialog(
            project, "pith generate", "What to generate:",
            relational = false,
            pathLabel = "New file path (relative to project root):"
        )
        if (!dialog.showAndGet()) return
        val relPath = dialog.path
        val prompt  = dialog.prompt
        if (relPath.isEmpty() || prompt.isEmpty()) return

        val fullPath = "$basePath/$relPath"

        // Agent backends may edit existing files while generating; unsaved
        // documents they touch would trigger the File Cache Conflict dialog.
        FileDocumentManager.getInstance().saveAllDocuments()

        runPith(e, listOf("generate", fullPath, "--prompt", prompt, "--apply") + dialog.contextArgs() + backend) {
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
