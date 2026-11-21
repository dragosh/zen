---
title: Quizzes
---
# Quizzes

Write a quiz in markdown and zen turns it into a form. The answers stay out of
the page: zen checks them when you press Check, so a quiz can't be solved by
reading the page source. Your score is kept on this Mac.

## Syntax

A `quiz` container holds numbered questions. Three kinds exist:

````markdown
:::quiz{id=http-basics title="HTTP basics" pass=70}
1. Which method is idempotent?
   - ( ) POST
   - (x) PUT
   - ( ) PATCH
   > PUT replaces the resource, so repeating it changes nothing.
2. Select the 2xx codes.
   - [x] 200
   - [x] 204
   - [ ] 301
3. What does TLS stand for? = Transport Layer Security
:::
````

| Write | Kind |
|---|---|
| `- ( )` and `- (x)` | one right option (radio buttons) |
| `- [ ]` and `- [x]` | several right options (checkboxes) |
| `question = answer` | free text; case and spacing are ignored |
| `question = /regex/` | free text matched by a pattern of up to 200 characters |
| `> text` under a question | the explanation, shown after you check |

Options on the container:

| Option | Meaning |
|---|---|
| `id` | names the stored results; keep it unique in a page |
| `title` | the heading above the quiz |
| `pass` | the score in percent that counts as a pass (default 70) |
| `attempts` | how many times you may try (default: no limit) |
| `shuffle` | mix up the order of the options |
| `reveal=each` | check one question at a time (default: all at the end) |

Your answers are saved while you work, and every attempt keeps its date and
score. The line above the quiz shows the best one.

## Try it

:::quiz{id=zen-basics title="Know your zen" pass=60 attempts=5}
1. Which language runs zen's backend?
   - ( ) Rust
   - (x) Go
   - ( ) Python
   > The window uses Wails, and the markdown engine is written in Go.
2. Which marker opens a container block in markdown?
   - ( ) ---
   - ( ) ###
   - (x) :::
   > A container starts and ends with three colons.
3. Which of these are built-in plugins?
   - [x] diagram
   - [x] polls
   - [ ] spreadsheet
   - [x] codeblock
   > Spreadsheets are not a plugin.
4. What is the name of the file that describes a plugin folder? = plugin.yml
   > Every plugin folder needs one.
5. Which kind of hook lets a plugin's page ask its backend a question? = /^(the )?action( hook)?$/
   > The page calls ctx.invoke, and the backend answers in an action.
:::

## Your results

Every quiz you took in this folder is listed here:

:::quiz-results
:::
