// WASM integration test — loads the built demo WASM via Node.js and
// exercises goRender for the index, a table page, sorting and BasePath-free
// links. Run: node cmd/demo/wasm_test.js docs
// Exit 0 on success, 1 on failure.

'use strict';

const fs = require('fs');
const path = require('path');
const { webcrypto } = require('crypto');

if (typeof globalThis.crypto === 'undefined') {
	globalThis.crypto = webcrypto;
}

const docsDir = path.resolve(process.argv[2] || path.join(__dirname, '..', '..', 'docs'));
require(path.join(docsDir, 'wasm_exec.js'));

let failures = 0;
let passes = 0;

function assert(cond, msg) {
	if (!cond) {
		console.error('  FAIL:', msg);
		failures++;
	} else {
		passes++;
	}
}

async function main() {
	const go = new Go();
	const buf = fs.readFileSync(path.join(docsDir, 'main.wasm'));
	const { instance } = await WebAssembly.instantiate(buf, go.importObject);
	go.run(instance); // resolves goRender registration then blocks in select{}

	// Give the Go side a beat to register goRender.
	for (let i = 0; i < 100 && typeof globalThis.goRender !== 'function'; i++) {
		await new Promise((r) => setTimeout(r, 50));
	}
	assert(typeof globalThis.goRender === 'function', 'goRender registered');

	const index = goRender('/');
	assert(index.includes('DB Explorer'), 'index has title');
	for (const t of ['authors', 'books', 'members', 'loans']) {
		assert(index.includes('>' + t + '</a>'), 'index lists ' + t);
	}
	assert(index.includes('Relationships'), 'index shows relationships');

	const books = goRender('/books?sort=title&dir=asc');
	assert(books.includes('Table: books'), 'books page renders');
	assert(books.includes('Foreign Keys'), 'books page has FKs');
	assert(books.includes('Indexes'), 'books page has indexes');
	assert(books.includes('Ancillary Justice'), 'books sorted asc starts with Ancillary');

	const loans = goRender('/loans?page=2');
	assert(loans.includes('Table: loans'), 'loans page 2 renders');
	assert(loans.includes('pagination'), 'loans paginates (120 rows)');

	const missing = goRender('/no_such_table');
	assert(missing.includes('Table Not Found'), 'unknown table handled');

	console.log('=== Results: ' + passes + ' passed, ' + failures + ' failed ===');
	process.exit(failures ? 1 : 0);
}

main().catch((err) => {
	console.error(err);
	process.exit(1);
});
