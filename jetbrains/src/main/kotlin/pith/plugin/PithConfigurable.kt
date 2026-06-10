package pith.plugin

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.util.ExecUtil
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.ui.ComboBox
import com.intellij.openapi.ui.Messages
import com.intellij.ui.components.JBCheckBox
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBPasswordField
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import javax.swing.JButton
import javax.swing.JComponent
import javax.swing.JPanel

class PithConfigurable : Configurable {

    // Display label -> stored mode value
    private val modes = linkedMapOf(
        "pith config (the CLI's own stored default)" to "config",
        "Agent (claude, codex — edits files itself)"  to "agent",
        "API (OpenRouter, OpenAI, Ollama — fast splice)" to "api"
    )

    private val binaryField = JBTextField()
    private val modeBox     = ComboBox(modes.keys.toTypedArray())
    private val agentField  = JBTextField()
    private val apiField    = JBTextField()
    private val modelField  = JBTextField()
    private val keyField    = JBPasswordField()
    private val previewBox  = JBCheckBox("Preview context && cost before sending AI edits")
    private val priceButton = JButton("Fetch rates for this model")

    init {
        // Write-through like the key field: runs `pith price <model>` via the
        // resolved binary, caching current rates so previews can show cost
        // offline — no terminal, no hunting for the executable.
        priceButton.addActionListener {
            val model = modelField.text.trim()
            if (model.isEmpty()) {
                Messages.showWarningDialog("Set an API model first.", "pith")
                return@addActionListener
            }
            try {
                val cmd = GeneralCommandLine(PithBinary.resolve(), "price", model)
                    .withCharset(Charsets.UTF_8)
                val out = ExecUtil.execAndGetOutput(cmd, 30_000)
                if (out.exitCode == 0) {
                    Messages.showInfoMessage(out.stdout.trim(), "pith — rates cached")
                } else {
                    Messages.showErrorDialog((out.stderr + out.stdout).trim(), "pith")
                }
            } catch (ex: Exception) {
                Messages.showErrorDialog("Couldn't run pith: ${ex.message}", "pith")
            }
        }
    }

    override fun getDisplayName() = "pith"

    override fun createComponent(): JComponent = FormBuilder.createFormBuilder()
        .addLabeledComponent(JBLabel("pith binary path (empty = bundled):"), binaryField)
        .addComponentToRightColumn(JBLabel("Empty uses the ${PithBinary.bundledHint()}."))
        .addLabeledComponent(JBLabel("AI backend:"), modeBox)
        .addLabeledComponent(JBLabel("Agent command:"), agentField)
        .addLabeledComponent(JBLabel("API (preset or URL):"), apiField)
        .addLabeledComponent(JBLabel("API model:"), modelField)
        .addComponentToRightColumn(priceButton)
        .addLabeledComponent(JBLabel("API key:"), keyField)
        .addComponentToRightColumn(JBLabel("Saved to pith's own config store on Apply — never kept in the IDE."))
        .addComponent(previewBox)
        .addComponentFillVertically(JPanel(), 0)
        .panel

    private fun selectedMode(): String = modes[modeBox.selectedItem as String] ?: "agent"

    /** The env-var name pith reads the key from, per the API preset. */
    private fun keyEnvFor(apiTarget: String): String? = when (apiTarget) {
        "openai"     -> "OPENAI_API_KEY"
        "openrouter" -> "OPENROUTER_API_KEY"
        "ollama"     -> null // local, no key
        else         -> "PITH_API_KEY"
    }

    override fun isModified(): Boolean {
        val s = PithSettings.getInstance().state
        return binaryField.text != s.pithBinary ||
               agentField.text  != s.agentCommand ||
               selectedMode()   != s.backendMode ||
               apiField.text    != s.apiTarget ||
               modelField.text  != s.apiModel ||
               previewBox.isSelected != s.previewBeforeSend ||
               keyField.password.isNotEmpty()
    }

    override fun apply() {
        val s = PithSettings.getInstance().state
        s.pithBinary   = binaryField.text.trim()
        s.agentCommand = agentField.text.trim()
        s.backendMode  = selectedMode()
        s.apiTarget    = apiField.text.trim()
        s.apiModel     = modelField.text.trim()
        s.previewBeforeSend = previewBox.isSelected

        // The key is write-through: handed once to `pith config set`, which owns
        // it from then on (file-permission-protected, masked, shared by every
        // editor). The IDE keeps nothing — the field clears after Apply.
        val key = String(keyField.password).trim()
        if (key.isNotEmpty()) {
            val env = keyEnvFor(s.apiTarget)
            if (env == null) {
                Messages.showInfoMessage("The '${s.apiTarget}' preset is local — it doesn't use an API key.", "pith")
            } else {
                try {
                    val p = ProcessBuilder(PithBinary.resolve(), "config", "set", env, key)
                        .redirectErrorStream(true).start()
                    val out = p.inputStream.bufferedReader().readText()
                    if (p.waitFor() != 0) {
                        Messages.showErrorDialog("pith config set failed:\n$out", "pith")
                    }
                } catch (ex: Exception) {
                    Messages.showErrorDialog("Couldn't run pith: ${ex.message}", "pith")
                }
            }
            keyField.setText("")
        }
    }

    override fun reset() {
        val s = PithSettings.getInstance().state
        binaryField.text = s.pithBinary
        agentField.text  = s.agentCommand
        modeBox.selectedItem = modes.entries.firstOrNull { it.value == s.backendMode }?.key
            ?: modes.keys.first()
        apiField.text   = s.apiTarget
        modelField.text = s.apiModel
        previewBox.isSelected = s.previewBeforeSend
        keyField.setText("")
    }
}
