---
title: Quizzes
---
# Quizzes

Write a quiz in markdown and Zenmark turns it into a form. The answers stay out of
the page: Zenmark checks them when you press Check, so a quiz can't be solved by
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

:::quiz{id=guide-basics title="Know this guide" pass=60 attempts=5}
1. Where does the sidebar get its pages from?
   - ( ) A table of contents file
   - (x) The links between pages, starting at README.md
   - ( ) Every file, in alphabetical order
   > A page appears under the first page that links to it.
2. Which marker opens a container block in markdown?
   - ( ) ---
   - ( ) ###
   - (x) :::
   > A container starts and ends with three colons.
3. Which of these are built-in plugins in this guide?
   - [x] diagram
   - [x] check
   - [ ] spreadsheet
   - [x] codeblock
   > Spreadsheets are not a plugin.
4. What is the name of the folder's settings file? = .zenmd.yml
   > It sits at the top of the folder; see Configuration.
5. Which panel lists a page's broken links? = /^(the )?(problems|inspector)( panel| tab)?$/
   > The Inspector's Problems tab lists them.
:::

## Your results

Every quiz you took in this folder is listed here:

:::quiz-results
:::
