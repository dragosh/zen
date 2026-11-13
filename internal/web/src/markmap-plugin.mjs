import { visit } from "unist-util-visit";
import { escapeScript } from "markmap-common";
import { Transformer } from "markmap-lib";
import { h } from "hastscript";

const transformer = new Transformer();
const getId = (
  (id) => () =>
    ++id
)(0);

function createMarkmap(content) {
  const { root, frontmatter } = transformer.transform(content);
  return [
    `<div id="markmap-${getId()}" class="gatsby-markmap">`,
    '<script type="text/markmap">',
    escapeScript(JSON.stringify({ data: root, options: frontmatter?.markmap })),
    "</script>",
    "</div>",
  ].join("");
}

export default function markmap() {
  /**
   * @param {import('mdast').Root} tree
   *   Tree.
   * @returns {undefined}
   *   Nothing.
   */
  return function (tree) {
    visit(tree, function (node) {
      if (
        node.type === "markmap" ||
        node.type === "markdown" ||
        node.meta === "markmap"
      ) {
        node.type = "html";
        node.value = createMarkmap(node.value);
      }
    });
  };
}
