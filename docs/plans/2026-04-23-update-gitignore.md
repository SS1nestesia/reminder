# Update Gitignore and Clean Repository Implementation Plan

> **For Antigravity:** REQUIRED WORKFLOW: Use `.agent/workflows/execute-plan.md` to execute this plan in single-flow mode.

**Goal:** Clean up the repository by updating `.gitignore` and removing accidentally tracked sensitive/temporary files.

**Architecture:** Update the root `.gitignore` with a comprehensive list of patterns and use `git rm --cached` to stop tracking files that should be ignored.

**Tech Stack:** Git, GitNexus

---

### Task 1: Update .gitignore

**Files:**
- Modify: [`.gitignore`](file:///Users/ilya/GolandProjects/reminder-bot/.gitignore)

**Step 1: Update .gitignore with comprehensive patterns**
Update the file with patterns for IDEs, Antigravity tools, Go temporary files, and database files.

**Step 2: Commit .gitignore**
```bash
git add .gitignore
git commit -m "chore: update .gitignore with comprehensive patterns"
```

### Task 2: Remove accidentally tracked files from index

**Files:**
- Modify: Git Index

**Step 1: Remove sensitive and temporary files from git index**
Stop tracking files that should be ignored.
```bash
git rm --cached .env .mcp.json .opencode.json .windsurfrules .cursorrules coverage.out coverage_func.txt reminder_bot.db
git rm -r --cached .cursor/ .claude/
```

**Step 2: Verify with GitNexus**
Run `gitnexus_detect_changes()` to verify the impact.

**Step 3: Commit removal**
```bash
git commit -m "chore: stop tracking sensitive and temporary files"
```

### Task 3: Final Verification and Re-indexing

**Step 1: Run GitNexus Analyze**
Update the GitNexus index.
```bash
/usr/local/bin/gitnexus analyze
```

**Step 2: Check status**
Run `git status` to ensure everything is clean.
