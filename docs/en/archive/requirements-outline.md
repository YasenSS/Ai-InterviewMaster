# InterviewMaster RAG AI Interview Full-Process Assistant — Requirements Document

> **Archived.** Historical requirements draft. See [`launch-redesign_0814.md`](../launch-redesign_0814.md) for the product boundaries and [`technical-plan_v1.md`](technical-plan_v1.md) for the historical engineering baseline.

## 1. Project Overview

### 1.1 Background

During job searching, candidates often face information asymmetry, subjective self-review, and interview question banks that are not sufficiently targeted.

### 1.2 Project Goals

Build an intelligent system based on a RAG architecture. Using the candidate’s resume, interview records, and real-time interview experiences from the internet, it provides **personalized, in-depth critique and precise prediction** for interview preparation.

## 2. Core Functional Requirements

### 2.1 Module 1: Deep Review and Critique of Interview Records (Interview Critique)

- **Inputs:** The user’s interview audio transcript, candidate resume, and corresponding job description (JD).
- **Functional points:**
  - **Interviewer-intent analysis:** Analyze the focus behind each interviewer question, such as technical depth, teamwork, or stress tolerance.
  - **Multidimensional scoring:** Score the candidate’s answer for logic, completeness, and technical accuracy on a scale of 0–100.
  - **Critical feedback:** Identify redundancy, ambiguity, or logical flaws in the answer.
  - **Wording optimization (Golden Answer):** Combine real project details from the resume to generate a smoother, more detailed reference answer based on the STAR method.

### 2.2 Module 2: Resume-Based Precise Question Prediction (Resume-Based Q&A Prediction)

- **Input:** A PDF/Word resume uploaded by the user.
- **Functional points:**
  - **Resume knowledge-graph construction:** Extract the technology stack, project highlights, and responsibility keywords from the resume.
  - **Assessment of examination-point weights:** Analyze which parts of the resume are most likely to be challenged by an interviewer, such as high-concurrency handling, outdated technology stacks, or employment gaps.
  - **Mock question bank:** Generate 3–5 progressively deeper follow-up questions for each project.

### 2.3 Module 3: Automated Company-Specific Interview-Experience Retrieval (Company-Specific Intel)

- **Inputs:** Target company name and target-position keywords.
- **Functional points:**
  - **Interview-experience crawling/retrieval:** Obtain interview experiences from the knowledge base, or through search APIs, covering platforms such as Niuke, Zhihu, and LeetCode over the past six months.
  - **Frequently asked question clustering:** Automatically summarize the company’s interview style, such as an emphasis on algorithms, foundational knowledge, or system/scenario design.
  - **Real-time question updates:** Extract the latest actual interview questions and store them in a local vector database.

---

## 3. Technical Architecture Design (RAG-Focused)

### 3.1 Data Layer (Data Ingestion)

- **Resumes/records:** Process them structurally with the Unstructured library and store them in the user’s personal vector space.
- **External interview experiences:** Crawler/API acquisition → cleaning → chunking → embedding → storage in a shared vector database (Milvus / Pinecone / Chroma).

### 3.2 Retrieval Layer (Retrieval)

- **Multi-route recall:** Combine **semantic retrieval** (vector search) and **keyword retrieval** (BM25) to ensure accurate retrieval of interview questions for a specific company and role.
- **Context compression:** Extract only interview-experience fragments relevant to the current resume project to reduce LLM token consumption.

### 3.3 Generation Layer (Generation)

- **Prompt engineering:**
  - *Role definition:* “You are a technical interviewer and career coach with 10 years of experience.”
  - *Few-shot learning:* Provide patterns for excellent answers.
- **LLM selection:** GPT-4o or Claude 3.5 Sonnet was recommended for their strong performance in logical critique and detail optimization.

## 4. Non-Functional Requirements

| **Dimension** | **Requirement description** |
| --- | --- |
| **Privacy protection** | A candidate’s resume and interview records are highly private; data desensitization or a local-storage option must be implemented. |
| **Response time** | The total retrieval + generation time should be controlled within 5–10 seconds to preserve the review experience. |
| **Accuracy** | Technical answers must cite real data from the resume and must not fabricate project experience (hallucination check). |

## 5. User Interaction Flow (User Workflow)

1. **Preparation:** The user uploads a resume → the system automatically analyzes it and generates a “resume core examination-point graph.”
2. **Preparation for interviews:** The user enters a “target company” → the system retrieves relevant interview experiences and combines them with the resume to generate a “predicted test.”
3. **Review:** After the interview, the user uploads the transcript → the system compares the “reference answer” with the actual answer → provides a score report and improved wording.

## 6. To-Do Items and Phased Roadmap

- **Phase 1 (MVP):** Implement resume parsing and resume-based AI question-prediction capabilities.
- **Phase 2 (RAG Integration):** Build a vector database and implement retrieval and answering for local/online interview experiences.
- **Phase 3 (Optimization):** Introduce a speech-to-text (ASR) module to support one-click interview-recording import and automatic critique.
