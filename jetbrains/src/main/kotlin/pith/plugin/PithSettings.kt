package pith.plugin

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage

@State(name = "PithSettings", storages = [Storage("pith.xml")])
@Service(Service.Level.APP)
class PithSettings : PersistentStateComponent<PithSettings.State> {

    data class State(
        var pithBinary: String = "pith",
        var agentCommand: String = "claude --dangerously-skip-permissions -p"
    )

    private var myState = State()

    override fun getState(): State = myState
    override fun loadState(state: State) { myState = state }

    companion object {
        fun getInstance(): PithSettings =
            ApplicationManager.getApplication().getService(PithSettings::class.java)
    }
}
