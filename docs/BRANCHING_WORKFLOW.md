# Git Branching Workflow

This document describes the branching strategy for the Glooscap project to support multiple developers working on different machines.

## Branch Structure

- **`main`**: Production-ready code. Only merged via PRs from `develop` or hotfix branches.
- **`develop`**: Integration branch for ongoing work. All feature branches merge here first.
- **`feature/*`**: Feature branches for specific work (e.g., `feature/fix-iskoces-build`, `feature/add-ui-service`).
- **`hotfix/*`**: Urgent fixes that need to go directly to `main` (bypass `develop`).

**Note:** We use `develop` instead of `development` because there's an existing `development/linux-operator-fixes` branch that would conflict.

## Workflow

### Starting New Work

1. **Update your local branches:**
   ```bash
   git fetch origin
   git checkout develop
   git pull origin develop
   ```

2. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Work on your feature:**
   - Make commits as you work
   - Keep commits focused and meaningful

### Sharing Work Between Machines

1. **On Machine A (where you made changes):**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **On Machine B (to continue work):**
   ```bash
   git fetch origin
   git checkout feature/your-feature-name
   ```

### Completing a Feature

1. **Push your feature branch:**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create a Pull Request:**
   - PR from `feature/your-feature-name` → `develop`
   - Get review and approval
   - Merge PR (squash or merge commit, as preferred)

3. **Clean up locally:**
   ```bash
   git checkout develop
   git pull origin develop
   git branch -d feature/your-feature-name
   ```

### Syncing with Remote

**Always fetch before starting work:**
```bash
git fetch origin
```

**Update develop branch:**
```bash
git checkout develop
git pull origin develop
```

**Update main (if needed):**
```bash
git checkout main
git pull origin main
```

## Best Practices

1. **Never commit directly to `main` or `develop`** - always use feature branches
2. **Always pull before pushing** to avoid conflicts
3. **Keep feature branches focused** - one feature per branch
4. **Use descriptive branch names** - `feature/fix-iskoces-build` not `feature/fix`
5. **Commit often** - small, focused commits are easier to review and revert
6. **Push feature branches regularly** when working across machines

## Handling Conflicts

If you encounter conflicts when pulling:

1. **Stash your changes:**
   ```bash
   git stash
   git pull origin develop
   git stash pop
   ```

2. **Resolve conflicts** in the affected files

3. **Test your changes** after resolving conflicts

4. **Commit the resolution:**
   ```bash
   git add <resolved-files>
   git commit -m "fix: Resolve merge conflicts with develop"
   ```

## Current Status

- **Active branches:** Check with `git branch -a`
- **Remote updates:** Run `git fetch origin` regularly
- **Uncommitted changes:** Use `git status` to check before switching branches

