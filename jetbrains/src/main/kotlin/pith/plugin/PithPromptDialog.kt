package pith.plugin

import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.ComboBox
import com.intellij.openapi.ui.DialogWrapper
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent

/**
 * The shared edit/generate input dialog. Context is two independent axes plus
 * a detail switch — they compose, they are not one flat list:
 *
 *   SCOPE  where resolution may look (none / file / folder / project)
 *   MODE   selective (only what the selection references) vs everything in scope
 *   DEPTH  how many reference hops to follow — selective mode only
 *   FULL   real implementations instead of outlines
 *
 * None = no scope = nothing else applies, enforced by control enablement.
 * Generate has no selection, so it gets scope only (always "everything").
 */
class PithPromptDialog(
    project: Project,
    dialogTitle: String,
    private val promptLabel: String,
    private val relational: Boolean, // true for edit (has a selection), false for generate
    private val pathLabel: String? = null,
) : DialogWrapper(project) {

    private val promptField = JBTextField(45)
    private val pathField   = if (pathLabel != null) JBTextField(45) else null

    private val scopes = if (relational)
        arrayOf("None", "This file", "This folder", "Whole project")
    else
        arrayOf("None", "Destination folder", "Whole project")
    private val scopeBox  = ComboBox(scopes)
    private val detailBox = ComboBox(arrayOf(
        "Full nearby, outlines deeper",
        "Outlines only",
        "Full everywhere",
    ))

    init {
        title = dialogTitle
        init()
        val sync = {
            detailBox.isEnabled = scopeBox.selectedIndex > 0 && relational
        }
        scopeBox.addActionListener { sync() }
        sync()
    }

    override fun createCenterPanel(): JComponent {
        val fb = FormBuilder.createFormBuilder()
        val pl = pathLabel
        if (pathField != null && pl != null) fb.addLabeledComponent(JBLabel(pl), pathField)
        fb.addLabeledComponent(JBLabel(promptLabel), promptField)
        fb.addLabeledComponent(JBLabel("Context scope:"), scopeBox)
        if (relational) {
            fb.addLabeledComponent(JBLabel("Detail:"), detailBox)
            fb.addComponentToRightColumn(
                JBLabel("pith follows what your selection references to the end of the chain — the preview shows what it found.")
            )
        }
        return fb.panel
    }

    override fun getPreferredFocusedComponent(): JComponent = pathField ?: promptField

    val prompt: String get() = promptField.text.trim()
    val path: String get() = pathField?.text?.trim() ?: ""

    /** Composes the axes into pith's --context level ([] when scope is None). */
    fun contextArgs(): List<String> {
        val scope = scopeBox.selectedIndex
        if (scope == 0) return emptyList()

        if (!relational) { // generate: positional outlines only
            return listOf("--context", if (scope == 1) "dir" else "project")
        }

        // Depth is not a user decision: the chain is always followed to its
        // natural end (caps + detail falloff keep that safe and cheap), and
        // the preview gate shows what it found before anything is sent.
        var level = "uses"
        if (scope == 2) level += ":dir"
        if (scope == 3) level += ":project"
        level += ":all"
        when (detailBox.selectedIndex) {
            0 -> level += ":full1" // full nearby, outlines deeper — relevance falloff
            2 -> level += ":full"
            // 1 = outlines only — no detail token
        }
        return listOf("--context", level)
    }
}
