# Contributing to {{title}}

This page says where a new page goes and how to write it. The four parts of
the site follow the [Diátaxis](https://diataxis.fr) approach, which is described
in full on its own website; what follows is a short version in our own words.

## Where does my page go?

Ask what the reader is doing when they open it.

1. They are **studying**, and want to gain skill: write a **tutorial**, in `tutorials/`.
2. They are **working**, and want to finish a task: write a **how-to guide**, in `how-to/`.
3. They are **working**, and need a fact such as a name, a limit or a default: write **reference**, in `reference/`.
4. They are **studying**, and want to know why: write an **explanation**, in `explanation/`.

If one page tries to do two of these, split it and link the halves.

## Add the page

1. Create a markdown file in the right folder. Use lower case and dashes in the name, such as `how-to/rotate-a-key.md`.
2. Start it with a single `#` heading. That heading becomes the title in the sidebar.
3. Link to it from the `index.md` of the same folder. zen builds the sidebar from links, so a page nobody links to ends up under "Other pages".
4. Run `zenmd build` and open the result to check the page and its links.

## Style

- Write in the second person and in the present tense.
- Put one idea in each paragraph and keep paragraphs short.
- Show commands and settings in fenced code blocks, and name the language so they get colour.
- Prefer a link to a repeat. Say a thing once, in the part where it belongs.
