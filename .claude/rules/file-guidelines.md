# File Guideline Comments

Every `.go` file MUST have a file guideline comment block at the top of the file, immediately after the `package` declaration.

## Format

```go
package example

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - FuncA() — brief description of what it does,
//     FuncB() — brief description of what it does.
//
// MUST NOT GO HERE:
//   - Category of code that doesn't belong (package/)
//   - Another category (package/)
//
// Q: Common question about this file?
// A: Short, direct answer pointing to the right place.
//
// Q: Another common question?
// A: Answer.
// ──────────────────────────────────────────────────────────────────
```

## Rules

- Place the block immediately after the `package` line, before any imports
- Use the box-drawing line (`// ──────────`) as top and bottom borders
- Start with `// FILE GUIDELINES`
- **BELONGS HERE** lists the functions/types in this file with a brief description of each
- **MUST NOT GO HERE** lists categories of code that might be confused as belonging here, with a pointer to where it actually lives
- **Q&A pairs** address common questions a developer might ask when looking at this file — where to add things, what the boundaries are, how to extend behavior
- Keep descriptions terse — one line per item
- Update the block when adding, removing, or renaming exported functions/types in the file
- When creating a new file, always include this block as part of the initial content
- When modifying an existing file that lacks this block, add it
