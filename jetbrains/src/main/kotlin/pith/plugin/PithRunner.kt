package pith.plugin

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.OSProcessHandler
import com.intellij.execution.process.ProcessAdapter
import com.intellij.execution.process.ProcessEvent
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.util.Key

object PithRunner {

    fun run(
        args: List<String>,
        workDir: String,
        onOutput: (String) -> Unit,
        onDone: () -> Unit
    ) {
        val binary = PithSettings.getInstance().state.pithBinary

        val cmd = GeneralCommandLine()
            .withExePath(binary)
            .withParameters(args)
            .withWorkDirectory(workDir)
            .withRedirectErrorStream(true)
            .withCharset(Charsets.UTF_8)

        val handler = OSProcessHandler(cmd)

        handler.addProcessListener(object : ProcessAdapter() {
            override fun onTextAvailable(event: ProcessEvent, outputType: Key<*>) {
                ApplicationManager.getApplication().invokeLater { onOutput(event.text) }
            }
            override fun processTerminated(event: ProcessEvent) {
                ApplicationManager.getApplication().invokeLater { onDone() }
            }
        })

        handler.startNotify()
    }
}
