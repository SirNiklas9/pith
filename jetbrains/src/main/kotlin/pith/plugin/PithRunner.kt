package pith.plugin

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.OSProcessHandler
import com.intellij.execution.process.ProcessAdapter
import com.intellij.execution.process.ProcessEvent
import com.intellij.execution.process.ProcessOutputTypes
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.util.Key

object PithRunner {

    /**
     * Runs pith with [args]. [onOutput] receives every chunk of output on the
     * EDT with isStdout=true only for real stdout — callers that capture
     * machine output (e.g. edit --raw) must ignore stderr/system chunks.
     */
    fun run(
        args: List<String>,
        workDir: String,
        onOutput: (text: String, isStdout: Boolean) -> Unit,
        onDone: () -> Unit
    ) {
        val binary = PithSettings.getInstance().state.pithBinary

        val cmd = GeneralCommandLine()
            .withExePath(binary)
            .withParameters(args)
            .withWorkDirectory(workDir)
            .withCharset(Charsets.UTF_8)

        val handler = OSProcessHandler(cmd)

        handler.addProcessListener(object : ProcessAdapter() {
            override fun onTextAvailable(event: ProcessEvent, outputType: Key<*>) {
                val isStdout = outputType == ProcessOutputTypes.STDOUT
                ApplicationManager.getApplication().invokeLater { onOutput(event.text, isStdout) }
            }
            override fun processTerminated(event: ProcessEvent) {
                ApplicationManager.getApplication().invokeLater { onDone() }
            }
        })

        handler.startNotify()
    }
}
