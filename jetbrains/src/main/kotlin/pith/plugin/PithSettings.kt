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
        var agentCommand: String = "claude --dangerously-skip-permissions -p",
        // "config" = pass no backend flags, pith's own stored config decides;
        // "agent" = pass --agent <agentCommand>; "api" = pass --api/--model.
        var backendMode: String = "agent",
        var apiTarget: String = "openrouter",
        var apiModel: String = "openai/gpt-4o-mini"
    )

    private var myState = State()

    override fun getState(): State = myState
    override fun loadState(state: State) { myState = state }

    /** The backend flags AI actions append, per the configured mode. */
    fun backendArgs(): List<String> = when (myState.backendMode) {
        "config" -> emptyList()
        "api"    -> listOf("--api", myState.apiTarget, "--model", myState.apiModel)
        else     -> listOf("--agent", myState.agentCommand)
    }

    companion object {
        fun getInstance(): PithSettings =
            ApplicationManager.getApplication().getService(PithSettings::class.java)
    }
}
