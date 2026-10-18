import { build } from 'esbuild';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
await mkdir('dist', { recursive: true });
await build({ entryPoints: ['src/client.js'], bundle: true, format: 'esm', minify: true, outfile: 'dist/client.js' });
const readCSS = await readFile('dist/client.css', 'utf8');
const readJS = (await readFile('dist/client.js', 'utf8')).replaceAll('</script', '<\\/script');
const html = (await readFile('public/index.html', 'utf8')).replace('/* BUILD_CSS */', () => readCSS).replace('/* BUILD_JS */', () => readJS);
await writeFile('dist/page.html', html);
await build({ entryPoints: ['src/index.js'], bundle: true, format: 'esm', minify: true, external: ['cloudflare:workers'], loader: { '.html': 'text' }, outfile: 'dist/worker.js' });
