# How to write a path

A path is one markdown file that uses the `course` layout. Each `##` heading
starts a step, and zen shows one step at a time.

## Add a path

1. Create a folder in `paths/`, such as `paths/my-subject/`, and a file `index.md` inside it.
2. Copy the frontmatter from [your first path](paths/first-path/index.md), and give the course its own `id`. The id names the saved progress, so keep it unique and do not change it later.
3. Write each step under a `##` heading. Keep a step to one idea.
4. Link the new path from the home page, so it shows up in the sidebar.

## What a step can hold

| You want | Write |
|---|---|
| a quiz | a `:::quiz` block with numbered questions |
| a checklist that unlocks the next step | a `:::check` block with `- [ ]` items |
| a picture or a diagram | an image, or a fenced `mermaid` block |

## Tips

- A step with a checklist stays locked until every box is ticked, so use it for the exercise, not for reading.
- Finish with a recap that lists what the learner can now do.
- Ask the same question in the quiz that the step answers, in the same words.
