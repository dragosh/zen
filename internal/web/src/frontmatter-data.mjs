import { visit, EXIT } from "unist-util-visit";
import yaml from "yaml";

export default function attacher(options = {}) {
	return function transformer(tree, vfile) {
		visit(tree, "yaml", visitor);

		function visitor(node) {
			vfile.data.frontmatter = yaml.parse(node.value);

			return EXIT;
		}

		if (
			vfile.data.frontmatter != null &&
			typeof options.transform === "function"
		) {
			const result = options.transform(vfile.data.frontmatter, vfile);
			if (typeof result.then === "function") {
				return result.then((transformed) => {
					vfile.data.frontmatter = transformed;
				});
			} else {
				vfile.data.frontmatter = result;
			}
		}

		return undefined;
	};
}
