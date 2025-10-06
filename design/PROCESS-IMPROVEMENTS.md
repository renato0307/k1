# Process Improvements for k1 Development

| Metadata | Value                      |
|----------|----------------------------|
| Date     | 2025-10-06                 |
| Author   | @renato0307                |
| Status   | `Proposed`                 |
| Type     | Process Guidelines         |

## Context

After conducting DDR-14 code quality analysis, we identified significant
technical debt that accumulated during rapid prototyping and feature
development. This document establishes process improvements to prevent
similar issues in future development.

## Quality Gates

### File Size Limits

**Trigger refactoring when:**
- Single file exceeds 500 lines (warning)
- Single file exceeds 800 lines (mandatory refactoring before new
  features)
- Single function exceeds 100 lines (warning)
- Single function exceeds 150 lines (mandatory refactoring)

**Claude Code responsibility**: Flag files approaching limits during
development.

### Test Coverage Requirements

**Minimum coverage thresholds:**
- New components: 70% coverage minimum
- Modified components: Cannot decrease coverage
- Critical paths (commands, data access): 80% coverage minimum

**Process:**
- Write tests DURING implementation, not after
- Run `make test-coverage` before considering feature complete
- Add test tasks to implementation plans

**Claude Code responsibility**: Remind about test coverage after
implementing any component over 100 lines.

### Code Duplication Limits

**Trigger refactoring when:**
- Same code pattern repeated 3+ times
- Copy-paste between files detected
- Similar functions differ only in parameters

**Claude Code responsibility**: Suggest abstraction/helper functions
when duplication detected.

### Complexity Limits

**Warning thresholds:**
- Function with 5+ levels of nesting
- Function with 10+ conditional branches
- State machine with 7+ states

**Claude Code responsibility**: Suggest decomposition when complexity
thresholds reached.

## Development Phases

### Phase 1: Planning
- Create PLAN-XX document with:
  - Feature goals
  - Architecture impact analysis
  - **Refactoring needs** (new section)
  - Test strategy
- Identify components that will grow significantly
- Plan refactoring if existing code needs cleanup

### Phase 2: Implementation
- Implement features incrementally
- Write tests alongside code (not after)
- Flag quality gate violations as they occur
- Update plan status as work progresses

### Phase 3: Quality Check (NEW)
**Required before marking plan complete:**
- Run `make test-coverage` and verify coverage targets
- Check for files exceeding size limits
- Review for code duplication
- Update CLAUDE.md if patterns changed

**Claude Code responsibility**: Create quality check checklist and
verify completion.

### Phase 4: Refactoring (NEW)
**Triggered by:**
- Quality gate violations
- Completing 2-3 major features without refactoring
- Code review feedback

**Process:**
- Create mini-DDR documenting issues (if architectural)
- Implement fixes with test coverage
- Verify no regressions with full test suite

## Architecture Review Cadence

### After Every Major Feature
**Quick check (5 minutes):**
- Largest file size?
- Overall test coverage?
- New duplication introduced?
- Any code smells evident?

**Claude Code responsibility**: Proactively perform this check and
report findings.

### After Every 2-3 Features
**Deep review (30 minutes):**
- Full codebase scan for code smells
- Identify architectural issues
- Prioritize refactoring tasks
- Optional: Create mini-DDR if significant issues found

**Claude Code responsibility**: Suggest architecture review when
appropriate.

### Quarterly (or Every 5-10 Features)
**Comprehensive audit (2 hours):**
- Full DDR-14 style analysis
- Update architecture documentation
- Plan major refactoring sprints

## Claude Code Behavioral Changes

### 1. Proactive Quality Warnings

**When implementing code, I will:**
- Flag when file reaches 500 lines: "Warning: This file is getting
  large (500+ lines). Consider refactoring after current feature."
- Flag when file reaches 800 lines: "STOP: This file exceeds 800
  lines. Refactoring required before new features."
- Suggest abstractions when detecting 3rd duplication
- Remind about tests when component exceeds 100 lines without tests

### 2. Plan Quality Sections

**Every plan will include:**
- **Refactoring needs**: Existing code that needs cleanup
- **Test strategy**: Specific tests to write, coverage targets
- **Quality risks**: Components that might exceed size limits
- **Post-implementation checklist**: Quality gates to verify

### 3. Post-Feature Reviews

**After completing major features, I will:**
- Run quality check (file sizes, coverage, duplication)
- Report findings proactively
- Suggest refactoring if needed
- Update CLAUDE.md with new patterns

### 4. Honest Progress Reporting

**During implementation:**
- Report technical debt being incurred
- Suggest pausing for refactoring when appropriate
- Don't hide quality issues to maintain velocity

## Specific Improvements for k1

### Immediate Actions (Before Next Feature)

1. **Establish test coverage baseline**:
   ```bash
   make test-coverage
   # Document current coverage
   # Set targets for improvement
   ```

2. **Create quality dashboard** (simple markdown file):
   ```markdown
   # k1 Quality Metrics

   | Metric | Current | Target | Status |
   |--------|---------|--------|--------|
   | Test coverage | 30% | 70% | ⚠️ Below target |
   | Largest file | 1257 lines | <500 lines | ❌ Needs refactoring |
   | Files >500 lines | 3 | 0 | ❌ Needs refactoring |
   | Known duplications | ~30 instances | 0 | ❌ Needs refactoring |
   ```

3. **Prioritize DDR-14 high-priority items**:
   - Split CommandBar (highest impact)
   - Extract constants
   - Remove dead code
   - Fix command injection

### Before Starting New Features

**Mandatory checklist:**
- [ ] DDR-14 high-priority items completed
- [ ] Test coverage >50% (interim target)
- [ ] CommandBar refactored to <500 lines per component
- [ ] Quality gates documented and agreed upon

### For Future Features

**Every plan must include:**
- Pre-refactoring needs section
- Test strategy with coverage targets
- Post-implementation quality checklist
- Maximum acceptable file sizes for new components

## Success Criteria

### Short Term (1 month)
- Zero files over 800 lines
- Test coverage >50%
- All high-priority DDR-14 items resolved

### Medium Term (3 months)
- Zero files over 500 lines
- Test coverage >70%
- Zero code injection vulnerabilities
- All medium-priority DDR-14 items resolved

### Long Term (6 months)
- Test coverage >80%
- Automated quality checks in CI/CD
- Regular architecture reviews (every 3 features)
- Zero known code smells

## Lessons Learned

**What we should have done differently:**

1. **After PLAN-04 completion**: Should have created mini-DDR reviewing
   the config-driven architecture and identifying refactoring needs

2. **When CommandBar hit 600 lines**: Should have paused and split it
   into focused components

3. **Before PLAN-05**: Should have added tests for CommandBar before
   extending it further

4. **After seeing duplication in transforms**: Should have immediately
   created shared abstraction instead of accepting it

5. **Throughout development**: Should have written tests alongside
   code, not deferred them

**Key insight**: Velocity without quality creates debt that eventually
slows you down more than taking time to refactor would have.

## Commitment

Going forward, I (Claude Code) commit to:

1. **Proactively flag quality issues** as they emerge during
   development
2. **Suggest refactoring pauses** when quality gates are approaching
3. **Include quality sections** in all plans
4. **Perform post-feature quality checks** and report findings
5. **Be honest about technical debt** being incurred
6. **Prioritize sustainability** over short-term velocity

The user should hold me accountable to these commitments. If I'm not
flagging issues proactively, please remind me of this document.
