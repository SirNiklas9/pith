package pith.plugin

import com.intellij.openapi.options.Configurable
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent
import javax.swing.JPanel

class PithConfigurable : Configurable {

    private val binaryField = JBTextField()
    private val agentField  = JBTextField()

    override fun getDisplayName() = "pith"

    override fun createComponent(): JComponent = FormBuilder.createFormBuilder()
        .addLabeledComponent(JBLabel("pith binary path:"), binaryField)
        .addLabeledComponent(JBLabel("Default agent command:"), agentField)
        .addComponentFillVertically(JPanel(), 0)
        .panel

    override fun isModified(): Boolean {
        val s = PithSettings.getInstance().state
        return binaryField.text != s.pithBinary || agentField.text != s.agentCommand
    }

    override fun apply() {
        val s = PithSettings.getInstance().state
        s.pithBinary    = binaryField.text.trim()
        s.agentCommand  = agentField.text.trim()
    }

    override fun reset() {
        val s = PithSettings.getInstance().state
        binaryField.text = s.pithBinary
        agentField.text  = s.agentCommand
    }
}
