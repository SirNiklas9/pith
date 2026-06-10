package pith.plugin

import com.intellij.openapi.options.Configurable
import com.intellij.openapi.ui.ComboBox
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
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

    override fun getDisplayName() = "pith"

    override fun createComponent(): JComponent = FormBuilder.createFormBuilder()
        .addLabeledComponent(JBLabel("pith binary path (empty = bundled):"), binaryField)
        .addComponentToRightColumn(JBLabel("Empty uses the ${PithBinary.bundledHint()}."))
        .addLabeledComponent(JBLabel("AI backend:"), modeBox)
        .addLabeledComponent(JBLabel("Agent command:"), agentField)
        .addLabeledComponent(JBLabel("API (preset or URL):"), apiField)
        .addLabeledComponent(JBLabel("API model:"), modelField)
        .addComponentFillVertically(JPanel(), 0)
        .panel

    private fun selectedMode(): String = modes[modeBox.selectedItem as String] ?: "agent"

    override fun isModified(): Boolean {
        val s = PithSettings.getInstance().state
        return binaryField.text != s.pithBinary ||
               agentField.text  != s.agentCommand ||
               selectedMode()   != s.backendMode ||
               apiField.text    != s.apiTarget ||
               modelField.text  != s.apiModel
    }

    override fun apply() {
        val s = PithSettings.getInstance().state
        s.pithBinary   = binaryField.text.trim()
        s.agentCommand = agentField.text.trim()
        s.backendMode  = selectedMode()
        s.apiTarget    = apiField.text.trim()
        s.apiModel     = modelField.text.trim()
    }

    override fun reset() {
        val s = PithSettings.getInstance().state
        binaryField.text = s.pithBinary
        agentField.text  = s.agentCommand
        modeBox.selectedItem = modes.entries.firstOrNull { it.value == s.backendMode }?.key
            ?: modes.keys.first()
        apiField.text   = s.apiTarget
        modelField.text = s.apiModel
    }
}
