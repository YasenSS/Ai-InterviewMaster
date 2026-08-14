param(
    [string]$BaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http

function New-ApiClient {
    $handler = [System.Net.Http.HttpClientHandler]::new()
    $handler.UseCookies = $true
    $handler.CookieContainer = [System.Net.CookieContainer]::new()
    $client = [System.Net.Http.HttpClient]::new($handler)
    $client.BaseAddress = [Uri]$BaseUrl
    return @{ Client = $client; Handler = $handler }
}

function Invoke-Api {
    param(
        [System.Net.Http.HttpClient]$Client,
        [string]$Method,
        [string]$Path,
        [object]$Body,
        [string]$AccessToken
    )
    $request = [System.Net.Http.HttpRequestMessage]::new(
        [System.Net.Http.HttpMethod]::new($Method),
        $Path
    )
    if ($AccessToken) {
        $request.Headers.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new(
            "Bearer",
            $AccessToken
        )
    }
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $request.Content = [System.Net.Http.StringContent]::new(
            $json,
            [System.Text.Encoding]::UTF8,
            "application/json"
        )
    }
    $response = $Client.SendAsync($request).GetAwaiter().GetResult()
    $text = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    $data = $null
    if ($text) {
        try { $data = $text | ConvertFrom-Json } catch { $data = $null }
    }
    return @{ Status = [int]$response.StatusCode; Data = $data; Text = $text }
}

function Assert-Status {
    param([hashtable]$Response, [int]$Expected, [string]$Step)
    if ($Response.Status -ne $Expected) {
        throw "$Step expected HTTP $Expected, got $($Response.Status): $($Response.Text)"
    }
}

function Put-PresignedObject {
    param([string]$PresignedUrl, [byte[]]$Bytes, [string]$ContentType)
    $request = [System.Net.Http.HttpRequestMessage]::new(
        [System.Net.Http.HttpMethod]::Put,
        [Uri]$PresignedUrl
    )
    $request.Content = [System.Net.Http.ByteArrayContent]::new($Bytes)
    $request.Content.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new($ContentType)
    $client = [System.Net.Http.HttpClient]::new()
    try {
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        if (-not $response.IsSuccessStatusCode) {
            $text = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            throw "presigned upload failed: $([int]$response.StatusCode) $text"
        }
    }
    finally {
        $client.Dispose()
    }
}

function Wait-Task {
    param(
        [System.Net.Http.HttpClient]$Client,
        [string]$TaskId,
        [string]$AccessToken,
        [string]$Step
    )
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        $response = Invoke-Api $Client "GET" "/api/v1/tasks/$TaskId" $null $AccessToken
        Assert-Status $response 200 $Step
        if ($response.Data.status -in @("succeeded", "failed")) {
            return $response
        }
        Start-Sleep -Milliseconds 500
    }
    throw "$Step did not reach a terminal state"
}

$runId = [Guid]::NewGuid().ToString("N").Substring(0, 12)
$password = "Backend-$runId!"
$accountA = New-ApiClient
$accountB = New-ApiClient

$health = Invoke-Api $accountA.Client "GET" "/api/v1/health" $null $null
Assert-Status $health 200 "health"
$ready = Invoke-Api $accountA.Client "GET" "/api/v1/ready" $null $null
Assert-Status $ready 200 "readiness"
if ($ready.Data.database -ne "ok" -or $ready.Data.redis -ne "ok") {
    throw "readiness did not verify database and Redis"
}

$registerA = Invoke-Api $accountA.Client "POST" "/api/v1/auth/register" @{
    email        = "backend-e2e-a-$runId@example.com"
    password     = $password
    display_name = "Backend E2E A"
} $null
Assert-Status $registerA 201 "register A"
$tokenA = $registerA.Data.access_token

$registerB = Invoke-Api $accountB.Client "POST" "/api/v1/auth/register" @{
    email        = "backend-e2e-b-$runId@example.com"
    password     = $password
    display_name = "Backend E2E B"
} $null
Assert-Status $registerB 201 "register B"
$tokenB = $registerB.Data.access_token

foreach ($removedPath in @(
    "/api/v1/job-descriptions",
    "/api/v1/question-sets",
    "/api/v1/tasks"
)) {
    $removed = Invoke-Api $accountA.Client "GET" $removedPath $null $tokenA
    Assert-Status $removed 404 "removed public route $removedPath"
}

$emptyInterviews = Invoke-Api $accountA.Client "GET" "/api/v1/interviews?page=1&page_size=20" $null $tokenA
Assert-Status $emptyInterviews 200 "empty interview records"
if ($emptyInterviews.Data.total -ne 0 -or $emptyInterviews.Data.items.Count -ne 0) {
    throw "new user must have an empty interview record list"
}

$resumeText = @"
Backend Engineer
Built Go services with PostgreSQL and Redis.
Improved production latency by 40 percent and documented incident reviews.
Designed idempotent asynchronous jobs with bounded retries and monitoring.
"@
$resumeBytes = [System.Text.Encoding]::UTF8.GetBytes($resumeText)
$upload = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/uploads" @{
    title        = "Backend Resume"
    file_name    = "resume.txt"
    content_type = "text/plain"
    size_bytes   = $resumeBytes.Length
} $tokenA
Assert-Status $upload 201 "create resume upload"
$resumeId = $upload.Data.resume_id
$versionId = $upload.Data.version_id
Put-PresignedObject $upload.Data.upload_url $resumeBytes "text/plain"

$completeUpload = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/$resumeId/versions/$versionId/complete" $null $tokenA
Assert-Status $completeUpload 202 "complete resume upload"
$parseTask = Wait-Task $accountA.Client $completeUpload.Data.task_id $tokenA "parse resume"
if ($parseTask.Data.status -ne "succeeded") {
    throw "resume parse failed: $($parseTask.Text)"
}

$invalidLanguage = Invoke-Api $accountA.Client "POST" "/api/v1/interviews" @{
    resume_id       = $resumeId
    primary_language = "Cobol"
    target_company  = "Example Corp"
} $tokenA
Assert-Status $invalidLanguage 400 "unsupported primary language"

$created = Invoke-Api $accountA.Client "POST" "/api/v1/interviews" @{
    resume_id                    = $resumeId
    primary_language             = "Go"
    target_company              = "Example Corp"
    question_duration_seconds   = 180
} $tokenA
Assert-Status $created 201 "create interview directly"
$interviewId = $created.Data.id
if ($created.Data.status -notin @("preparing", "active") -or -not $created.Data.task_id) {
	throw "new interview did not enter preparation or become active"
}
if ($null -ne $created.Data.PSObject.Properties["question_set"] -or
    $null -ne $created.Data.PSObject.Properties["job_description"]) {
    throw "internal preparation resources leaked into the interview response"
}

$crossUserInterview = Invoke-Api $accountB.Client "GET" "/api/v1/interviews/$interviewId" $null $tokenB
Assert-Status $crossUserInterview 404 "cross-user interview access"

$session = $null
for ($attempt = 0; $attempt -lt 120; $attempt++) {
    $session = Invoke-Api $accountA.Client "GET" "/api/v1/interviews/$interviewId" $null $tokenA
    Assert-Status $session 200 "wait for interview preparation"
    if ($session.Data.status -in @("active", "failed")) { break }
    Start-Sleep -Milliseconds 500
}
if ($session.Data.status -ne "active") {
    throw "interview preparation failed: $($session.Text)"
}
if ($session.Data.turns.Count -ne 1 -or $session.Data.turns[0].state -ne "answering") {
    throw "only the first real question should be exposed after preparation"
}

$firstOrdinal = $session.Data.current_ordinal
for ($turnIndex = 0; $turnIndex -lt 15 -and $session.Data.status -eq "active"; $turnIndex++) {
    $ordinal = $session.Data.current_ordinal
    $answer = "For turn $ordinal I would establish context and measurable goals, implement the change with idempotency and monitoring, then verify the result with production metrics."
    $answered = Invoke-Api $accountA.Client "PUT" "/api/v1/interviews/$interviewId/turns/$ordinal/answer" @{
        answer = $answer
    } $tokenA
    Assert-Status $answered 200 "answer adaptive turn $ordinal"
    $session = $answered
}
if ($session.Data.status -ne "completed") {
    throw "adaptive interview did not complete within its bounded turn budget: $($session.Text)"
}

$overwrite = Invoke-Api $accountA.Client "PUT" "/api/v1/interviews/$interviewId/turns/$firstOrdinal/answer" @{
    answer = "A different answer that must not overwrite the historical record."
} $tokenA
Assert-Status $overwrite 409 "prevent answer overwrite"

$report = $null
for ($attempt = 0; $attempt -lt 120; $attempt++) {
    $report = Invoke-Api $accountA.Client "GET" "/api/v1/interviews/$interviewId/report" $null $tokenA
    Assert-Status $report 200 "poll interview report"
    if ($report.Data.status -in @("completed", "degraded", "failed")) { break }
    Start-Sleep -Milliseconds 500
}
if ($report.Data.status -notin @("completed", "degraded")) {
    throw "interview report failed: $($report.Text)"
}
if ($report.Data.turns.Count -ne $session.Data.turns.Count) {
    throw "report does not contain the complete interview transcript"
}
foreach ($turn in $report.Data.turns) {
    if (-not $turn.question -or -not $turn.answer -or
        $null -eq $turn.score -or -not $turn.critique -or -not $turn.golden_answer) {
        throw "report turn is missing question, answer, score, critique, or golden answer"
    }
}

$records = Invoke-Api $accountA.Client "GET" "/api/v1/interviews?page=1&page_size=20&status=completed" $null $tokenA
Assert-Status $records 200 "list interview records"
if ($records.Data.total -lt 1 -or $records.Data.items[0].primary_language -ne "Go" -or
    $records.Data.items[0].target_company -ne "Example Corp") {
    throw "interview record list is missing the completed interview configuration"
}

$resumeDelete = Invoke-Api $accountA.Client "DELETE" "/api/v1/resumes/$resumeId" $null $tokenA
Assert-Status $resumeDelete 409 "protect resume used by interview history"
if ($resumeDelete.Text -match "question_set|题集") {
    throw "resume deletion leaked the internal candidate-question resource"
}

$crossUserTask = Invoke-Api $accountB.Client "GET" "/api/v1/tasks/$($completeUpload.Data.task_id)" $null $tokenB
Assert-Status $crossUserTask 404 "cross-user task access"
$dashboard = Invoke-Api $accountA.Client "GET" "/api/v1/dashboard/summary" $null $tokenA
Assert-Status $dashboard 200 "dashboard summary"
if ($dashboard.Data.counts.completed_interviews -lt 1 -or $dashboard.Data.score_trend.Count -lt 1) {
    throw "dashboard did not count the completed scored interview"
}

Write-Output "backend e2e passed (run_id=$runId)"
