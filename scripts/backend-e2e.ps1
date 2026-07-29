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
        $data = $text | ConvertFrom-Json
    }
    return @{
        Status  = [int]$response.StatusCode
        Data    = $data
        Text    = $text
        Headers = $response.Headers
    }
}

function Assert-Status {
    param([hashtable]$Response, [int]$Expected, [string]$Step)
    if ($Response.Status -ne $Expected) {
        throw "$Step expected HTTP $Expected, got $($Response.Status): $($Response.Text)"
    }
}

function Put-PresignedObject {
    param(
        [string]$PresignedUrl,
        [byte[]]$Bytes,
        [string]$ContentType
    )
    $signedUri = [Uri]$PresignedUrl
    $request = [System.Net.Http.HttpRequestMessage]::new(
        [System.Net.Http.HttpMethod]::Put,
        $signedUri
    )
    $request.Content = [System.Net.Http.ByteArrayContent]::new($Bytes)
    $request.Content.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new($ContentType)
    $client = [System.Net.Http.HttpClient]::new()
    $response = $client.SendAsync($request).GetAwaiter().GetResult()
    if (-not $response.IsSuccessStatusCode) {
        $text = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        throw "presigned upload failed: $([int]$response.StatusCode) $text"
    }
    $client.Dispose()
}

$runId = [Guid]::NewGuid().ToString("N").Substring(0, 12)
$password = "Backend-$runId!"
$newPassword = "Backend-New-$runId!"
$accountA = New-ApiClient
$accountB = New-ApiClient

$health = Invoke-Api $accountA.Client "GET" "/api/v1/health" $null $null
Assert-Status $health 200 "health"
$ready = Invoke-Api $accountA.Client "GET" "/api/v1/ready" $null $null
Assert-Status $ready 200 "readiness"
if ($ready.Data.database -ne "ok" -or $ready.Data.redis -ne "ok") {
    throw "readiness did not verify database and Redis"
}
$unauthorized = Invoke-Api $accountA.Client "GET" "/api/v1/me" $null $null
Assert-Status $unauthorized 401 "unauthenticated request"
if ($unauthorized.Data.code -ne "AUTH_REQUIRED" -or -not $unauthorized.Data.request_id) {
    throw "unauthenticated error is missing its stable code or request ID"
}

$registerA = Invoke-Api $accountA.Client "POST" "/api/v1/auth/register" @{
    email        = "backend-e2e-a-$runId@example.com"
    password     = $password
    display_name = "Backend E2E A"
} $null
Assert-Status $registerA 201 "register A"
$tokenA = $registerA.Data.access_token
if ($registerA.Data.expires_in -gt 900) {
    throw "access token is not short-lived"
}
$oldRefresh = $accountA.Handler.CookieContainer.GetCookies([Uri]"$BaseUrl/api/v1/auth").Item(
    "interviewmaster_refresh"
).Value
if (-not $oldRefresh) {
    throw "register did not set the refresh cookie"
}
$duplicateRegister = Invoke-Api $accountA.Client "POST" "/api/v1/auth/register" @{
    email        = "backend-e2e-a-$runId@example.com"
    password     = $password
    display_name = "Duplicate"
} $null
Assert-Status $duplicateRegister 409 "duplicate registration"

$refreshA = Invoke-Api $accountA.Client "POST" "/api/v1/auth/refresh" $null $null
Assert-Status $refreshA 200 "refresh A"
$tokenA = $refreshA.Data.access_token
$newRefresh = $accountA.Handler.CookieContainer.GetCookies([Uri]"$BaseUrl/api/v1/auth").Item(
    "interviewmaster_refresh"
).Value
if (-not $newRefresh -or $newRefresh -eq $oldRefresh) {
    throw "refresh token was not rotated"
}

$replayHandler = [System.Net.Http.HttpClientHandler]::new()
$replayHandler.UseCookies = $false
$replayClient = [System.Net.Http.HttpClient]::new($replayHandler)
$replayClient.BaseAddress = [Uri]$BaseUrl
$replayRequest = [System.Net.Http.HttpRequestMessage]::new(
    [System.Net.Http.HttpMethod]::Post,
    "/api/v1/auth/refresh"
)
$replayRequest.Headers.Add("Cookie", "interviewmaster_refresh=$oldRefresh")
$replayResponse = $replayClient.SendAsync($replayRequest).GetAwaiter().GetResult()
if ([int]$replayResponse.StatusCode -ne 401) {
    throw "rotated refresh token replay was not rejected"
}

$registerB = Invoke-Api $accountB.Client "POST" "/api/v1/auth/register" @{
    email        = "backend-e2e-b-$runId@example.com"
    password     = $password
    display_name = "Backend E2E B"
} $null
Assert-Status $registerB 201 "register B"
$tokenB = $registerB.Data.access_token

$emptyDashboard = Invoke-Api $accountB.Client "GET" "/api/v1/dashboard/summary" $null $tokenB
Assert-Status $emptyDashboard 200 "empty dashboard"
if ($emptyDashboard.Data.counts.resumes -ne 0 -or
    $null -ne $emptyDashboard.Data.PSObject.Properties["average_score"] -or
    $emptyDashboard.Data.score_trend.Count -ne 0) {
    throw "new-user dashboard must contain real empty values and omit average_score"
}
foreach ($emptyListPath in @(
    "/api/v1/resumes",
    "/api/v1/job-descriptions",
    "/api/v1/question-sets",
    "/api/v1/interviews",
    "/api/v1/tasks"
)) {
    $emptyList = Invoke-Api $accountB.Client "GET" $emptyListPath $null $tokenB
    Assert-Status $emptyList 200 "empty list $emptyListPath"
    if ($emptyList.Data.total -ne 0 -or $emptyList.Data.items.Count -ne 0) {
        throw "new-user list was not an empty paginated collection: $emptyListPath"
    }
}

$updateMe = Invoke-Api $accountA.Client "PATCH" "/api/v1/me" @{
    display_name = "Backend E2E A Updated"
} $tokenA
Assert-Status $updateMe 200 "update me"
$me = Invoke-Api $accountA.Client "GET" "/api/v1/me" $null $tokenA
Assert-Status $me 200 "get me"
if ($me.Data.display_name -ne "Backend E2E A Updated") {
    throw "profile update was not persisted"
}

$job = Invoke-Api $accountA.Client "POST" "/api/v1/job-descriptions" @{
    company = "Example Corp"
    title   = "Backend Engineer"
    content = "Build Go services with PostgreSQL, Redis, Docker, and distributed system design practices."
} $tokenA
Assert-Status $job 201 "create job description"
$jobId = $job.Data.id

$jobDetail = Invoke-Api $accountA.Client "GET" "/api/v1/job-descriptions/$jobId" $null $tokenA
Assert-Status $jobDetail 200 "get job description"
$jobUpdate = Invoke-Api $accountA.Client "PATCH" "/api/v1/job-descriptions/$jobId" @{
    company = "Example Corp Updated"
    content = "Build reliable Go services with PostgreSQL, Redis, Docker, observability, and distributed systems."
} $tokenA
Assert-Status $jobUpdate 200 "update job description"
if ($jobUpdate.Data.company -ne "Example Corp Updated" -or $jobUpdate.Data.capabilities.Count -lt 1) {
    throw "job description update did not re-extract capabilities"
}

$jobPage = Invoke-Api $accountA.Client "GET" "/api/v1/job-descriptions?page=1&page_size=5" $null $tokenA
Assert-Status $jobPage 200 "list job descriptions"
if ($jobPage.Data.total -lt 1 -or $null -eq $jobPage.Data.items) {
    throw "job description pagination is incomplete"
}
$invalidPage = Invoke-Api $accountA.Client "GET" "/api/v1/job-descriptions?page=0&page_size=101" $null $tokenA
Assert-Status $invalidPage 400 "invalid pagination"

$crossUserJob = Invoke-Api $accountB.Client "GET" "/api/v1/job-descriptions/$jobId" $null $tokenB
Assert-Status $crossUserJob 404 "cross-user job access"

$resumeText = @"
Backend Engineer
Built Go services with PostgreSQL and Redis.
Improved production latency by 40 percent and documented incident reviews.
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
if (([Uri]$upload.Data.upload_url).Host -ne "localhost") {
    throw "container API did not return a browser-reachable object upload URL"
}
Put-PresignedObject $upload.Data.upload_url $resumeBytes "text/plain"

$completeUpload = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/$resumeId/versions/$versionId/complete" $null $tokenA
Assert-Status $completeUpload 202 "complete resume upload"
$taskId = $completeUpload.Data.task_id
$completeUploadAgain = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/$resumeId/versions/$versionId/complete" $null $tokenA
Assert-Status $completeUploadAgain 202 "repeat complete resume upload"
if ($completeUploadAgain.Data.task_id -ne $taskId) {
    throw "repeated upload completion did not reuse its active task"
}

$task = $null
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $task = Invoke-Api $accountA.Client "GET" "/api/v1/tasks/$taskId" $null $tokenA
    Assert-Status $task 200 "get parse task"
    if ($task.Data.status -in @("succeeded", "failed")) {
        break
    }
    Start-Sleep -Milliseconds 500
}
if ($task.Data.status -ne "succeeded") {
    throw "resume parse task did not succeed: $($task.Text)"
}

$resumePage = Invoke-Api $accountA.Client "GET" "/api/v1/resumes?status=completed&page=1&page_size=5" $null $tokenA
Assert-Status $resumePage 200 "list resumes"
if ($resumePage.Data.total -lt 1 -or $resumePage.Data.items.Count -lt 1) {
    throw "resume pagination or filtering is incomplete"
}
$crossUserResume = Invoke-Api $accountB.Client "GET" "/api/v1/resumes/$resumeId" $null $tokenB
Assert-Status $crossUserResume 404 "cross-user resume access"
$renameResume = Invoke-Api $accountA.Client "PATCH" "/api/v1/resumes/$resumeId" @{
    title = "Backend Resume Updated"
} $tokenA
Assert-Status $renameResume 200 "rename resume"

$resume = Invoke-Api $accountA.Client "GET" "/api/v1/resumes/$resumeId" $null $tokenA
Assert-Status $resume 200 "get resume"
if ($resume.Data.status -ne "completed" -or $resume.Data.facts.Count -lt 1) {
    throw "resume details do not include completed parse facts"
}
$parsedResumeTime = [DateTimeOffset]::MinValue
if ($resume.Data.title -ne "Backend Resume Updated" -or
    -not [DateTimeOffset]::TryParse($resume.Data.updated_at, [ref]$parsedResumeTime)) {
    throw "resume details do not contain the renamed title and RFC3339 timestamps"
}

$reparse = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/$resumeId/reparse" $null $tokenA
Assert-Status $reparse 202 "reparse resume"
$reparseTask = $null
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $reparseTask = Invoke-Api $accountA.Client "GET" "/api/v1/tasks/$($reparse.Data.task_id)" $null $tokenA
    Assert-Status $reparseTask 200 "poll reparse task"
    if ($reparseTask.Data.status -in @("succeeded", "failed")) {
        break
    }
    Start-Sleep -Milliseconds 500
}
if ($reparseTask.Data.status -ne "succeeded") {
    throw "resume reparse did not succeed: $($reparseTask.Text)"
}
$partialDashboard = Invoke-Api $accountA.Client "GET" "/api/v1/dashboard/summary" $null $tokenA
Assert-Status $partialDashboard 200 "partial dashboard"
if ($partialDashboard.Data.counts.resumes -lt 1 -or
    $partialDashboard.Data.counts.job_descriptions -lt 1 -or
    $partialDashboard.Data.counts.interviews -ne 0 -or
    $null -ne $partialDashboard.Data.PSObject.Properties["average_score"]) {
    throw "partial dashboard did not reflect only the resources created so far"
}

$missingObject = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/uploads" @{
    title        = "Missing Object Resume"
    file_name    = "missing.txt"
    content_type = "text/plain"
    size_bytes   = 12
} $tokenA
Assert-Status $missingObject 201 "create missing-object upload"
$missingComplete = Invoke-Api $accountA.Client "POST" (
    "/api/v1/resumes/{0}/versions/{1}/complete" -f
    $missingObject.Data.resume_id,
    $missingObject.Data.version_id
) $null $tokenA
Assert-Status $missingComplete 404 "complete upload with missing object"
$missingDelete = Invoke-Api $accountA.Client "DELETE" (
    "/api/v1/resumes/{0}" -f $missingObject.Data.resume_id
) $null $tokenA
Assert-Status $missingDelete 204 "delete missing-object resume"

$questionSet = Invoke-Api $accountA.Client "POST" "/api/v1/question-sets" @{
    resume_id         = $resumeId
    job_description_id = $jobId
    target_role       = "Backend Engineer"
} $tokenA
Assert-Status $questionSet 201 "create question set"
$questionSetId = $questionSet.Data.id
$questionSetPage = Invoke-Api $accountA.Client "GET" (
    "/api/v1/question-sets?resume_id={0}&page=1&page_size=5" -f $resumeId
) $null $tokenA
Assert-Status $questionSetPage 200 "list question sets"
$questionSetDetail = Invoke-Api $accountA.Client "GET" "/api/v1/question-sets/$questionSetId" $null $tokenA
Assert-Status $questionSetDetail 200 "get question set"
$crossUserQuestionSet = Invoke-Api $accountB.Client "GET" "/api/v1/question-sets/$questionSetId" $null $tokenB
Assert-Status $crossUserQuestionSet 404 "cross-user question-set access"

$replacementQuestions = @(
    @{
        ordinal = 1
        question = "Describe one backend project."
        intent = "Project experience"
        expected_points = @("Context", "Action", "Result")
        follow_up_hint = "What was your personal contribution?"
    },
    @{
        ordinal = 2
        question = "How do you diagnose a production performance issue?"
        intent = "Problem solving"
        expected_points = @("Metrics", "Diagnosis", "Verification")
    },
    @{
        ordinal = 3
        question = "How do you design a reliable asynchronous task?"
        intent = "System design"
        expected_points = @("Idempotency", "Retries", "Observability")
    }
)
$updatedSet = Invoke-Api $accountA.Client "PATCH" "/api/v1/question-sets/$questionSetId" @{
    questions = $replacementQuestions
} $tokenA
Assert-Status $updatedSet 200 "replace question set"
if ($updatedSet.Data.question_count -ne 3) {
    throw "question set replacement did not persist three questions"
}
$invalidReplacement = Invoke-Api $accountA.Client "PATCH" "/api/v1/question-sets/$questionSetId" @{
    questions = @(
        @{
            ordinal = 2
            question = "Invalid ordinal"
            intent = "Rollback test"
            expected_points = @("Must not persist")
        }
    )
} $tokenA
Assert-Status $invalidReplacement 400 "invalid question-set replacement"
$setAfterRollback = Invoke-Api $accountA.Client "GET" "/api/v1/question-sets/$questionSetId" $null $tokenA
Assert-Status $setAfterRollback 200 "load question set after rollback"
if ($setAfterRollback.Data.question_count -ne 3) {
    throw "invalid full replacement changed the persisted question set"
}

$regenerated = Invoke-Api $accountA.Client "POST" "/api/v1/question-sets/$questionSetId/regenerate" @{
    target_role = "Senior Backend Engineer"
} $tokenA
Assert-Status $regenerated 201 "regenerate question set"
if ($regenerated.Data.source_question_set_id -ne $questionSetId) {
    throw "regenerated question set did not record its source"
}
$regeneratedId = $regenerated.Data.id
$deleteRegenerated = Invoke-Api $accountA.Client "DELETE" "/api/v1/question-sets/$regeneratedId" $null $tokenA
Assert-Status $deleteRegenerated 204 "delete unused regenerated question set"
$deleteRegeneratedAgain = Invoke-Api $accountA.Client "DELETE" "/api/v1/question-sets/$regeneratedId" $null $tokenA
Assert-Status $deleteRegeneratedAgain 204 "repeat delete question set"

$interview = Invoke-Api $accountA.Client "POST" "/api/v1/interviews" @{
    resume_id                = $resumeId
    question_set_id          = $questionSetId
    job_description_id       = $jobId
    title                    = "Backend Interview"
    question_duration_seconds = 180
} $tokenA
Assert-Status $interview 201 "create interview"
$interviewId = $interview.Data.id
if ($interview.Data.turns.Count -ne 3 -or $interview.Data.turns[0].state -ne "answering") {
    throw "interview did not initialize its turn snapshot"
}
if (-not $interview.Data.started_at -or $interview.Data.question_duration_seconds -ne 180) {
    throw "interview response is missing server timer fields"
}
$interviewPage = Invoke-Api $accountA.Client "GET" "/api/v1/interviews?status=active&page=1&page_size=5" $null $tokenA
Assert-Status $interviewPage 200 "list interviews"
if ($interviewPage.Data.total -lt 1) {
    throw "interview list did not include the active interview"
}
$crossUserInterview = Invoke-Api $accountB.Client "GET" "/api/v1/interviews/$interviewId" $null $tokenB
Assert-Status $crossUserInterview 404 "cross-user interview access"

$snapshotEdit = Invoke-Api $accountA.Client "PATCH" "/api/v1/question-sets/$questionSetId" @{
    questions = @($replacementQuestions[0])
} $tokenA
Assert-Status $snapshotEdit 200 "edit question set after interview"
$interviewAfterSetEdit = Invoke-Api $accountA.Client "GET" "/api/v1/interviews/$interviewId" $null $tokenA
Assert-Status $interviewAfterSetEdit 200 "load interview snapshot"
if ($interviewAfterSetEdit.Data.turns.Count -ne 3 -or
    $interviewAfterSetEdit.Data.started_at -ne $interview.Data.started_at -or
    $interviewAfterSetEdit.Data.duration_seconds -lt 0) {
    throw "question set edit changed the historical interview snapshot"
}

$answerOne = Invoke-Api $accountA.Client "PUT" "/api/v1/interviews/$interviewId/turns/1/answer" @{
    answer = "I owned a Go service from requirements through monitoring and reduced API latency by 40 percent."
} $tokenA
Assert-Status $answerOne 200 "answer turn one"
$answerOneAgain = Invoke-Api $accountA.Client "PUT" "/api/v1/interviews/$interviewId/turns/1/answer" @{
    answer = "I owned the Go service end to end, added monitoring, and reduced p95 latency by 40 percent."
} $tokenA
Assert-Status $answerOneAgain 200 "overwrite turn one"
if ($answerOneAgain.Data.turns[0].answer -notlike "*p95*") {
    throw "specific-turn answer overwrite did not persist"
}

$skipTwo = Invoke-Api $accountA.Client "POST" "/api/v1/interviews/$interviewId/turns/2/skip" $null $tokenA
Assert-Status $skipTwo 200 "skip turn two"
$skipTwoAgain = Invoke-Api $accountA.Client "POST" "/api/v1/interviews/$interviewId/turns/2/skip" $null $tokenA
Assert-Status $skipTwoAgain 200 "repeat skip turn two"

$legacyAnswerThree = Invoke-Api $accountA.Client "POST" "/api/v1/interviews/$interviewId/answer" @{
    answer = "Use idempotency keys, bounded retries, and terminal failure states."
} $tokenA
Assert-Status $legacyAnswerThree 200 "legacy answer endpoint"
$answerThree = Invoke-Api $accountA.Client "PUT" "/api/v1/interviews/$interviewId/turns/3/answer" @{
    answer = "Use idempotency keys, exponential backoff, terminal failure states, structured errors, and metrics."
} $tokenA
Assert-Status $answerThree 200 "answer turn three"

$incomplete = Invoke-Api $accountA.Client "POST" "/api/v1/interviews/$interviewId/complete" @{
    confirm_incomplete = $false
} $tokenA
Assert-Status $incomplete 409 "complete without confirming skipped turn"
if ($incomplete.Data.code -ne "INTERVIEW_HAS_UNANSWERED_TURNS") {
    throw "incomplete interview returned the wrong error code"
}

$completed = Invoke-Api $accountA.Client "POST" "/api/v1/interviews/$interviewId/complete" @{
    confirm_incomplete = $true
} $tokenA
Assert-Status $completed 200 "complete interview"
if ($completed.Data.status -ne "completed") {
    throw "interview did not enter completed state"
}
$completedAgain = Invoke-Api $accountA.Client "POST" "/api/v1/interviews/$interviewId/complete" @{
    confirm_incomplete = $true
} $tokenA
Assert-Status $completedAgain 200 "repeat complete interview"

$writeAfterComplete = Invoke-Api $accountA.Client "PUT" "/api/v1/interviews/$interviewId/turns/1/answer" @{
    answer = "This write must be rejected after completion."
} $tokenA
Assert-Status $writeAfterComplete 409 "write after completion"

$reportRequestOne = [System.Net.Http.HttpRequestMessage]::new(
    [System.Net.Http.HttpMethod]::Get,
    "/api/v1/interviews/$interviewId/report"
)
$reportRequestOne.Headers.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new(
    "Bearer",
    $tokenA
)
$reportRequestTwo = [System.Net.Http.HttpRequestMessage]::new(
    [System.Net.Http.HttpMethod]::Get,
    "/api/v1/interviews/$interviewId/report"
)
$reportRequestTwo.Headers.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new(
    "Bearer",
    $tokenA
)
$reportFutureOne = $accountA.Client.SendAsync($reportRequestOne)
$reportFutureTwo = $accountA.Client.SendAsync($reportRequestTwo)
$reportHttpOne = $reportFutureOne.GetAwaiter().GetResult()
$reportHttpTwo = $reportFutureTwo.GetAwaiter().GetResult()
$reportOne = @{
    Status = [int]$reportHttpOne.StatusCode
    Text   = $reportHttpOne.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    Data   = $null
}
$reportTwo = @{
    Status = [int]$reportHttpTwo.StatusCode
    Text   = $reportHttpTwo.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    Data   = $null
}
Assert-Status $reportOne 200 "generate report concurrently"
Assert-Status $reportTwo 200 "reuse report concurrently"
$reportOne.Data = $reportOne.Text | ConvertFrom-Json
$reportTwo.Data = $reportTwo.Text | ConvertFrom-Json
$reportThree = Invoke-Api $accountA.Client "GET" "/api/v1/interviews/$interviewId/report" $null $tokenA
Assert-Status $reportThree 200 "reuse persisted report"
if ($reportOne.Data.id -ne $reportTwo.Data.id -or
    $reportOne.Data.id -ne $reportThree.Data.id -or
    $reportOne.Data.turns.Count -ne 3 -or
    $reportOne.Data.turns[1].score -ne 0) {
    throw "concurrent report generation was not reused or lacks turn snapshots"
}

$corruptResumeBytes = [System.Text.Encoding]::UTF8.GetBytes("   ")
Put-PresignedObject $upload.Data.upload_url $corruptResumeBytes "text/plain"
$failedReparse = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/$resumeId/reparse" $null $tokenA
Assert-Status $failedReparse 202 "start failing reparse"
$failedReparseTask = $null
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $failedReparseTask = Invoke-Api $accountA.Client "GET" "/api/v1/tasks/$($failedReparse.Data.task_id)" $null $tokenA
    Assert-Status $failedReparseTask 200 "poll failing reparse"
    if ($failedReparseTask.Data.status -eq "failed") {
        break
    }
    Start-Sleep -Milliseconds 500
}
if ($failedReparseTask.Data.status -ne "failed") {
    throw "corrupted resume reparse did not reach a terminal failure"
}
$resumeAfterFailedReparse = Invoke-Api $accountA.Client "GET" "/api/v1/resumes/$resumeId" $null $tokenA
Assert-Status $resumeAfterFailedReparse 200 "load resume after failed reparse"
if ($resumeAfterFailedReparse.Data.status -ne "failed" -or
    $resumeAfterFailedReparse.Data.facts.Count -lt 1 -or
    -not $resumeAfterFailedReparse.Data.parse_error) {
    throw "failed reparse did not preserve old facts and expose a safe error summary"
}

$questionSetDelete = Invoke-Api $accountA.Client "DELETE" "/api/v1/question-sets/$questionSetId" $null $tokenA
Assert-Status $questionSetDelete 409 "delete in-use question set"
$resumeDelete = Invoke-Api $accountA.Client "DELETE" "/api/v1/resumes/$resumeId" $null $tokenA
Assert-Status $resumeDelete 409 "delete in-use resume"

$deleteJob = Invoke-Api $accountA.Client "DELETE" "/api/v1/job-descriptions/$jobId" $null $tokenA
Assert-Status $deleteJob 204 "delete job description"
$historyAfterJobDelete = Invoke-Api $accountA.Client "GET" "/api/v1/interviews/$interviewId" $null $tokenA
Assert-Status $historyAfterJobDelete 200 "load history after job deletion"

$taskPage = Invoke-Api $accountA.Client "GET" "/api/v1/tasks?page=1&page_size=5" $null $tokenA
Assert-Status $taskPage 200 "list tasks"
$filteredTaskPage = Invoke-Api $accountA.Client "GET" "/api/v1/tasks?status=succeeded&type=resume.parse&page=1&page_size=5" $null $tokenA
Assert-Status $filteredTaskPage 200 "filter tasks"
$crossUserTask = Invoke-Api $accountB.Client "GET" "/api/v1/tasks/$taskId" $null $tokenB
Assert-Status $crossUserTask 404 "cross-user task access"
$dashboard = Invoke-Api $accountA.Client "GET" "/api/v1/dashboard/summary" $null $tokenA
Assert-Status $dashboard 200 "dashboard summary"
if ($dashboard.Data.counts.completed_interviews -lt 1 -or
    $null -eq $dashboard.Data.average_score -or
    $dashboard.Data.score_trend.Count -lt 1) {
    throw "dashboard did not count the completed interview"
}

$invalidResumeBytes = [System.Text.Encoding]::UTF8.GetBytes("   ")
$failedUpload = Invoke-Api $accountA.Client "POST" "/api/v1/resumes/uploads" @{
    title        = "Retryable Failed Resume"
    file_name    = "empty.txt"
    content_type = "text/plain"
    size_bytes   = $invalidResumeBytes.Length
} $tokenA
Assert-Status $failedUpload 201 "create failing resume upload"
Put-PresignedObject $failedUpload.Data.upload_url $invalidResumeBytes "text/plain"
$failedComplete = Invoke-Api $accountA.Client "POST" (
    "/api/v1/resumes/{0}/versions/{1}/complete" -f
    $failedUpload.Data.resume_id,
    $failedUpload.Data.version_id
) $null $tokenA
Assert-Status $failedComplete 202 "start failing parse task"
$failedTask = $null
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $failedTask = Invoke-Api $accountA.Client "GET" (
        "/api/v1/tasks/{0}" -f $failedComplete.Data.task_id
    ) $null $tokenA
    Assert-Status $failedTask 200 "poll failing parse task"
    if ($failedTask.Data.status -eq "failed") {
        break
    }
    Start-Sleep -Milliseconds 500
}
if ($failedTask.Data.status -ne "failed" -or -not $failedTask.Data.error.code) {
    throw "failed task did not expose a structured safe error"
}
$retry = Invoke-Api $accountA.Client "POST" (
    "/api/v1/tasks/{0}/retry" -f $failedComplete.Data.task_id
) $null $tokenA
Assert-Status $retry 202 "retry failed task"
if ($retry.Data.task_id -eq $failedComplete.Data.task_id) {
    throw "task retry did not create a new task"
}
$retryTask = Invoke-Api $accountA.Client "GET" (
    "/api/v1/tasks/{0}" -f $retry.Data.task_id
) $null $tokenA
Assert-Status $retryTask 200 "load retried task"
if ($retryTask.Data.retry_of_task_id -ne $failedComplete.Data.task_id) {
    throw "retried task did not record its source task"
}
$retryAgain = Invoke-Api $accountA.Client "POST" (
    "/api/v1/tasks/{0}/retry" -f $failedComplete.Data.task_id
) $null $tokenA
Assert-Status $retryAgain 202 "repeat retry failed task"
if ($retryAgain.Data.task_id -ne $retry.Data.task_id -and
    $retryTask.Data.status -in @("pending", "running")) {
    throw "repeated retry did not reuse the active retry task"
}

$wrongPasswordChange = Invoke-Api $accountA.Client "POST" "/api/v1/auth/change-password" @{
    current_password = "Wrong-$runId"
    new_password     = $newPassword
} $tokenA
Assert-Status $wrongPasswordChange 400 "change password with wrong current password"
$changePassword = Invoke-Api $accountA.Client "POST" "/api/v1/auth/change-password" @{
    current_password = $password
    new_password     = $newPassword
} $tokenA
Assert-Status $changePassword 204 "change password"

$logout = Invoke-Api $accountA.Client "POST" "/api/v1/auth/logout" $null $null
Assert-Status $logout 204 "logout"
$logoutAgain = Invoke-Api $accountA.Client "POST" "/api/v1/auth/logout" $null $null
Assert-Status $logoutAgain 204 "repeat logout"

$oldLogin = Invoke-Api $accountA.Client "POST" "/api/v1/auth/login" @{
    email    = "backend-e2e-a-$runId@example.com"
    password = $password
} $null
Assert-Status $oldLogin 401 "login with old password"
$newLogin = Invoke-Api $accountA.Client "POST" "/api/v1/auth/login" @{
    email    = "backend-e2e-a-$runId@example.com"
    password = $newPassword
} $null
Assert-Status $newLogin 200 "login with changed password"

Write-Output "backend e2e passed (run_id=$runId)"
