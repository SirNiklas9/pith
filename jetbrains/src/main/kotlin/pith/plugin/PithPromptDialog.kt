package pith.plugin

import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.ComboBox
import com.intellij.openapi.ui.DialogWrapper
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent

/**
 * The shared edit/generate input dialog: an instruction field plus a context
 * selector (pith's --context levels, "none" by default — context stays opt-in
 * per invocation). Generate also gets a path field via [pathLabel].
 */
class PithPromptDialog(
    project: Project,
    dialogTitle: String,
    private val promptLabel: String,
    private val contextLevels: Map<String, String>, // display label -> --context value ("" = none)
    private val pathLabel: String? = null,
) : DialogWrapper(project) {

    private val promptField  = JBTextField(45)
    private val pathField    = if (pathLabel != null) JBTextField(45) else null
    private val contextBox   = ComboBox(contextLevels.keys.toTypedArray())

    init {
        title = dialogTitle
        init()
    }

    override fun createCenterPanel(): JComponent {
        val fb = FormBuilder.createFormBuilder()
        val pl = pathLabel
        if (pathField != null && pl != null) fb.addLabeledComponent(JBLabel(pl), pathField)
        fb.addLabeledComponent(JBLabel(promptLabel), promptField)
        fb.addLabeledComponent(JBLabel("Context:"), contextBox)
        return fb.panel
    }

    override fun getPreferredFocusedComponent(): JComponent = pathField ?: promptField

    val prompt: String get() = promptField.text.trim()
    val path: String get() = pathField?.text?.trim() ?: ""

    /** Extra args for the chosen context level ([] when none). */
    fun contextArgs(): List<String> {
        val level = contextLevels[contextBox.selectedItem as String] ?: ""
        return if (level.isEmpty()) emptyList() else listOf("--context", level)
    }

    companion object {
        val EDIT_CONTEXTS = linkedMapOf(
            "None — just the selection" to "",
            "Uses — what the selection references (outlines, this folder)" to "uses:dir",
            "Uses full — their implementations" to "uses:dir:full",
            "Uses deep — follow the chain 3 hops (implementations)" to "uses:dir:3:full",
            "Around — this file's outline" to "around",
            "File — this file's full source" to "file",
            "Dir — folder outline" to "dir",
            "Project — whole-project outline" to "project",
        )
        val GENERATE_CONTEXTS = linkedMapOf(
            "None — just the prompt" to "",
            "Dir — destination folder's outline" to "dir",
            "Project — whole-project outline" to "project",
        )
    }
}
