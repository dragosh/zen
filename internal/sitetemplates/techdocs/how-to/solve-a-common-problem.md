# How to solve a common problem

Use this guide when you need to fix the most common problem people hit with
{{title}}. It assumes you finished the
[getting started tutorial](../tutorials/getting-started.md).

*This page is an example. Replace it with a real task from your project.*

## Before you start

Name what must be true before the first step. For example: you can run the
tool, and you have the file you want to change.

## Steps

1. Find the setting that causes the problem. The [reference](../reference/settings.md) lists every one.
2. Change its value and save the file.
3. Run the command again.

```sh
tool run --verbose
```

4. Read the last lines of the output. A line that starts with `ok` means it worked.

## If it did not work

- **The command is not found.** Check that the tool is on your path, then try again.
- **The output has an error line.** Copy the line into a search of the reference, which lists each message.

## Related

To see why the setting exists, read [how it works](../explanation/how-it-works.md).
