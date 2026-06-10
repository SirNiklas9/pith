package pith.plugin

import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.DialogWrapper
import com.intellij.ui.components.JBCheckBox
import com.intellij.ui.components.JBScrollPane
import com.intellij.util.ui.JBUI
import java.awt.BorderLayout
import java.awt.Font
import javax.swing.JComponent
import javax.swing.JPanel
import javax.swing.JTextArea

/**
 * The pre-send gate: shows pith's --dry-run report (what the context resolved
 * to, hop by hop, and the byte/token budget) with Send/Cancel. Ticking
 * "don't ask again" flips the previewBeforeSend setting off, so future edits
 * go straight to the backend.
 */
class PithPreviewDialog(project: Project, report: String) : DialogWrapper(project) {

    private val dontAsk = JBCheckBox("Don't ask again — always send directly (Settings ▸ pith re-enables)")
    private val text = JTextArea(report).apply {
        isEditable = false
        font = Font(Font.MONOSPACED, Font.PLAIN, 12)
        margin = JBUI.insets(8)
    }

    init {
        title = "pith — preview before send"
        setOKButtonText("Send")
        init()
    }

    override fun createCenterPanel(): JComponent {
        val panel = JPanel(BorderLayout(0, 8))
        val scroll = JBScrollPane(text)
        scroll.preferredSize = JBUI.size(640, 320)
        panel.add(scroll, BorderLayout.CENTER)
        panel.add(dontAsk, BorderLayout.SOUTH)
        return panel
    }

    /** True when the user ticked the don't-ask-again box (and pressed Send). */
    fun dontAskAgain(): Boolean = dontAsk.isSelected
}
