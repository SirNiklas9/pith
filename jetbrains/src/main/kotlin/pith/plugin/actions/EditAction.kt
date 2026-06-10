package pith.plugin.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.command.CommandProcessor
import com.intellij.openapi.command.WriteCommandAction
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
        val backend = PithSettings.getInstance().backendArgs()

        val doc   = FileDocumentManager.getInstance().getDocument(vFile) ?: return
        val sel   = editor.selectionModel
        val start = doc.getLineNumber(sel.selectionStart) + 1
        val end   = doc.getLineNumber(sel.selectionEnd)   + 1

        val prompt = Messages.showInputDialog(
            project, "Edit instruction (lines $start–$end):", "pith edit", null
        ) ?: return

        // Save EVERYTHING, not just this document: an agent backend has latitude
        // to touch files beyond the selection, and an externally-rewritten file
        // only reloads silently when its document has no unsaved changes —
        // otherwise the IDE raises the File Cache Conflict dialog.
        FileDocumentManager.getInstance().saveAllDocuments()

        // --raw, not --apply: a completion backend prints the new region to
        // stdout and never touches the file, so the plugin can splice it into
        // the document itself — disk changes only through IntelliJ, which makes
        // a File Cache Conflict impossible. An agent backend ignores --raw and
        // writes the file directly; the mtime watcher below catches that case.
        val args    = listOf("edit", file, "--range", "$start:$end", "--prompt", prompt, "--raw") + backend
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
        val done    = AtomicBoolean(false)
        val applied = AtomicBoolean(false)
        val rawBuf  = StringBuilder()

        // AGENT path: the agent wrote the file on disk — inject its content into
        // the document (inside a command so undo works).
        //
        // The File Cache Conflict dialog fires on modification stamps alone;
        // the platform never compares content (MemoryDiskConflictResolver). So
        // the moment our setText marks the document unsaved, a VFS refresh of
        // the agent's disk write raises the dialog even though memory == disk.
        // Sequence that makes the trigger unreachable: write the bytes VFS
        // believes in back to disk, sync-refresh while the document is still
        // clean (identical content + clean doc = always silent), THEN do the
        // undoable setText and save onto a no-longer-stale file.
        fun applyFromDisk(): Boolean {
            val mtime = try {
                Files.getLastModifiedTime(filePath).toMillis()
            } catch (ex: Exception) { return false }
            if (mtime == initialMtime) return false
            if (!applied.compareAndSet(false, true)) return true
            try {
                Thread.sleep(80) // let the write flush
                val newContent = Files.readString(filePath).replace("\r\n", "\n").replace("\r", "\n")
                ApplicationManager.getApplication().invokeLater {
                    try {
                        if (!FileDocumentManager.getInstance().isDocumentUnsaved(doc)) {
                            Files.write(filePath, vFile.contentsToByteArray())
                            vFile.refresh(false, false)
                        }
                        // (document unsaved here = the user typed mid-run; the
                        // conflict is then genuine and the dialog is correct)
                        ApplicationManager.getApplication().runWriteAction {
                            CommandProcessor.getInstance().executeCommand(project, {
                                if (doc.text != newContent) doc.setText(newContent)
                            }, "pith edit", null)
                            FileDocumentManager.getInstance().saveDocument(doc)
                        }
                        PithToolWindowFactory.print(project, "\n[pith] applied — Ctrl+Z to undo\n")
                    } catch (ex: Exception) {
                        PithToolWindowFactory.print(project, "\n[pith] reload failed: ${ex.message}\n", true)
                    }
                }
            } catch (ex: Exception) {
                PithToolWindowFactory.print(project, "\n[pith] reload failed: ${ex.message}\n", true)
            }
            return true
        }

        Thread {
            var elapsed = 0
            while (elapsed < 120_000) {
                if (applyFromDisk()) break
                if (done.get()) { applyFromDisk(); break }
                Thread.sleep(50)
                elapsed += 50
            }
        }.also { it.isDaemon = true; it.start() }

        PithRunner.run(
            args     = args,
            workDir  = workDir,
            onOutput = { text, isStdout ->
                PithToolWindowFactory.print(project, text)
                if (isStdout) rawBuf.append(text) // only real stdout can be the region
            },
            onDone   = {
                done.set(true)
                // COMPLETION path: file untouched, the new region is on stdout —
                // replace the selected lines in the editor. Native undo, no disk
                // interplay, instant.
                val mtimeNow = try {
                    Files.getLastModifiedTime(filePath).toMillis()
                } catch (ex: Exception) { initialMtime }
                if (mtimeNow == initialMtime && rawBuf.isNotBlank() && applied.compareAndSet(false, true)) {
                    val newRegion = rawBuf.toString().replace("\r\n", "\n").trimEnd('\n')
                    val startOff  = doc.getLineStartOffset((start - 1).coerceIn(0, doc.lineCount - 1))
                    val endOff    = doc.getLineEndOffset((end - 1).coerceIn(0, doc.lineCount - 1))
                    WriteCommandAction.runWriteCommandAction(project) {
                        doc.replaceString(startOff, endOff, newRegion)
                    }
                    ApplicationManager.getApplication().runWriteAction {
                        FileDocumentManager.getInstance().saveDocument(doc)
                    }
                    PithToolWindowFactory.print(project, "\n[pith] applied — Ctrl+Z to undo\n")
                }
            }
        )
    }
}
