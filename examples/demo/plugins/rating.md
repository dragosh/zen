---
title: Ratings
---
# Ratings

A `rating` block asks one question and you pick 1 to 5 stars. Your rating is kept
on this Mac, so a guide or a [course](../layouts/course.md) can end with it, and
you can see later what you rated.

## Syntax

````markdown
:::rating{id=guide title="How useful was this page?"}
:::
````

Options on the container:

| Option | Meaning |
|---|---|
| `id` | names the stored rating; required, and unique in the page |
| `title` | the question (default "Rate this page") |
| `max` | the number of stars, 3 to 10 (default 5) |
| `labels` | one label per star, written `Poor\|Fair\|Good`, shown beside the stars |

Text inside the block shows under the question.

## Try it

:::rating{id=guide-rating title="How useful was this page?" labels="Not at all|A little|Useful|Very useful|Essential"}
Press a star, or use the arrow keys, Home and End. Clear removes your rating.
:::

## Your ratings

Every page you rated in this folder is listed here:

:::rating-results
:::
