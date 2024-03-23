import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import remarkGfm from "remark-gfm";
import rehypeStringify from "rehype-stringify";
import rehypeSanitize, {defaultSchema} from 'rehype-sanitize'
import rehypeKatex from 'rehype-katex'
import rehypePrism from "rehype-prism-plus";;
import remarkFrontmatter from 'remark-frontmatter';
import rehypeMermaid from 'rehype-mermaid';
import remarkFrontmatterData from './frontmatter-data.mjs';
import myPlugin from './my-plugin.mjs';
import calloutPlugin from './callout-plugin.mjs';
import yourtubePlugin from './youtube-plugin.mjs';
import remarkDirective from 'remark-directive'
import remarkMath from 'remark-math'
import mermaid from 'mermaid';

import './style.css';

mermaid.initialize({ startOnLoad: true })

const ready = (function () {
	if (document.readyState === "complete") {
		return Promise.resolve();
	}
	return new Promise(function (resolve) {
		window.addEventListener("load", resolve);
	});
})();

ready.then(async () => {
	const externalLink = document.querySelectorAll("a[target='_blank']");

	externalLink.forEach((el) => {
		el.addEventListener("click", (ev) => {
			console.log(el.href);

			ev.stopImmediatePropagation();
			ev.preventDefault();
			open(el.href).then(Promise.resolve);
			return false;
		});
	});


	// const quitBtn = document.getElementById("quit");
	// quitBtn.addEventListener("click", () => quit().then(Promise.resolve));
});

// disable context menu
// window.addEventListener("contextmenu", (ev) => ev.preventDefault());

window.reload = () => {
	window.location.href = window.location.href;
};

export const render = async (entryFile) => {
	let res = await fetch(entryFile);
	// // get the response body (the method explained below)
	let md = await res.text();
	let html = await unified()
		.use(remarkParse)
		.use(remarkDirective)
		.use(myPlugin)
		.use(yourtubePlugin)
		.use(calloutPlugin)
		.use(remarkFrontmatterData)
		.use(remarkGfm) // Support GFM (tables, autolinks, tasklists, strikethrough).
    .use(remarkFrontmatter, ['yaml'])
		.use(remarkMath)
		.use(remarkRehype, { fragment: true })
		.use(rehypeSanitize, {
			...defaultSchema,
				tagNames:[
					...defaultSchema.tagNames,
					'aside',
				],
    		attributes: {
					...defaultSchema.attributes,
					// The `language-*` regex is allowed by default.
					code: [['className', /^language-./, 'math-inline', 'math-display']],
					aside: [['className', /(warning|note|tip|info|admonition)/, '']]
    	}
		})
		.use(rehypeKatex)
		.use(rehypePrism, {ignoreMissing : true})
		.use(rehypeMermaid)
		.use(rehypeStringify)

		.process(md);

    const frontmatter = html.data.frontmatter
    // console.log('Html', html.value);
    console.log('Frontmatter', frontmatter);
	// if(! frontmatter?.id) {
	// 	html = "Missing id"
	// }

// 	// const Editor = toastui.Editor;

// 	const { codeSyntaxHighlight } = Editor.plugin;

// 	const editor = new Editor({
// 		el: document.querySelector('#editor'),
// 		height: '500px',
// 		initialEditType: 'markdown',
// 		initialValue: md,
// 		previewStyle: 'tab',
// 		theme: 'dark',
// 		plugins: [codeSyntaxHighlight]
// 	});

// 	editor.getMarkdown();

// 	console.log("editor", editor)

	document.getElementById("markdown").innerHTML = html;
}

// export const render = async (entryFile) => {
// 	let res = await fetch(entryFile);
// 	// // get the response body (the method explained below)
// 	let md = await res.text();
// 	Editor
//         .make()
//         .config(ctx => {
//           ctx.set(rootCtx, '#app')
//           ctx.set(defaultValueCtx, md)
//         })
//         // .config(nord)
// 				// .use(remarkParse)
// 				// .use(remarkFrontmatter, ['yaml'])
// 				// .use(remarkFrontmatterData)
// 				// .use(remarkRehype, { fragment: true })
// 				// .use(rehypePrism)
// 				// .use(rehypeMermaid)
// 				// .use(rehypeStringify)
//         .use(commonmark)
//         .create()
// }
