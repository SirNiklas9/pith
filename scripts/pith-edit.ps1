param(
    [string]$file,
    [string]$start,
    [string]$end,
    [string]$pith   = "pith",
    [string]$agent  = "",
    [string]$api    = "",
    [string]$model  = "",
    [string]$context = ""
)

Add-Type -AssemblyName Microsoft.VisualBasic
$prompt = [Microsoft.VisualBasic.Interaction]::InputBox(
    "Edit instruction (lines $start-$end):",
    "pith edit — $([System.IO.Path]::GetFileName($file))"
)
if ($prompt -eq "") { exit 0 }

$args = @("edit", $file, "--range", "${start}:${end}", "--prompt", $prompt, "--apply")
if ($agent  -ne "") { $args += @("--agent",   $agent)  }
if ($api    -ne "") { $args += @("--api",      $api)    }
if ($model  -ne "") { $args += @("--model",    $model)  }
if ($context -ne "") { $args += @("--context", $context) }

& $pith @args
