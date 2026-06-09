package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.command.CommandProcessor
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.wm.ToolWindowManager
import pith.plugin.PithRunner
import pith.plugin.PithSettings
import pith.plugin.PithToolWindowFactory
import java.nio.file.Files
import java.nio.file.Paths
import java.util.concurrent.atomic.AtomicBoolean

class EditAction : PithAction("Edit Selection...", "AI edit of the selected region") {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val vFile   = e.getData(CommonDataKeys.VIRTUAL_FILE) ?: return
        val editor  = e.getData(CommonDataKeys.EDITOR) ?: return
        val file    = vFile.path
        val agent   = PithSettings.getInstance().state.agentCommand

        val doc   = FileDocumentManager.getInstance().getDocument(vFile) ?: return
        val sel   = editor.selectionModel
        val start = doc.getLineNumber(sel.selectionStart) + 1
        val end   = doc.getLineNumber(sel.selectionEnd)   + 1

        val prompt = Messages.showInputDialog(
            project, "Edit instruction (lines $start–$end):", "pith edit", null
        ) ?: return

        FileDocumentManager.getInstance().saveDocument(doc)

        val args    = listOf("edit", file, "--range", "$start:$end", "--prompt", prompt, "--agent", agent, "--apply")
        val workDir = project.basePath ?: return

        ToolWindowManager.getInstance(project).getToolWindow("pith")?.show()
        PithToolWindowFactory.clear(project)
        PithToolWindowFactory.print(project, "pith ${args.joinToString(" ")}\n\n")

        val filePath     = Paths.get(file)
        val initialMtime = try {
            Files.getLastModifiedTime(filePath).toMillis()
        } catch (ex: Exception) {
            PithToolWindowFactory.print(project, "[pith] error: can't stat $file: ${ex.message}\n", true)
            return
        }
        val done = AtomicBoolean(false)

        // Poll the file's mtime every 50ms on a background thread. The agent writes
        // the file mid-run; we detect the write and inject the new content into the
        // document inside a command so undo/redo work. (Watching mtime beats waiting
        // for the process: on Windows the agent's child processes inherit pith's
        // stdout handle and keep the pipe open past the edit.)
        Thread {
            var elapsed = 0
            while (!done.get() && elapsed < 120_000) {
                Thread.sleep(50)
                elapsed += 50
                val mtime = try {
                    Files.getLastModifiedTime(filePath).toMillis()
                } catch (ex: Exception) { break }
                if (mtime != initialMtime && done.compareAndSet(false, true)) {
                    try {
                        Thread.sleep(80) // let the write flush
                        val newContent = Files.readString(filePath).replace("\r\n", "\n").replace("\r", "\n")
                        ApplicationManager.getApplication().invokeLater {
                            ApplicationManager.getApplication().runWriteAction {
                                CommandProcessor.getInstance().executeCommand(project, {
                                    doc.setText(newContent)
                                }, "pith edit", null)
                            }
                            PithToolWindowFactory.print(project, "\n[pith] applied — Ctrl+Z to undo\n")
                        }
                    } catch (ex: Exception) {
                        PithToolWindowFactory.print(project, "\n[pith] reload failed: ${ex.message}\n", true)
                    }
                }
            }
        }.also { it.isDaemon = true; it.start() }

        PithRunner.run(
            args     = args,
            workDir  = workDir,
            onOutput = { text -> PithToolWindowFactory.print(project, text) },
            onDone   = { done.set(true) }
        )
    }
}
