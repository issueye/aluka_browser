package webview

// injectedScript 在每个页面文档创建时注入：
//  1. 拦截 window.open 与 target=_blank / 中键 / Ctrl+点击 链接，
//     转为 open_tab 消息由宿主新开标签页；
//  2. 在 DOMContentLoaded / load 时上报 state（URL + 标题）。
const injectedScript = `
(function() {
	const origOpen = window.open;
	window.open = function(url, target, features) {
		if (url) {
			try {
				const fullUrl = new URL(url, window.location.href).href;
				window.chrome.webview.postMessage(JSON.stringify({ type: 'open_tab', url: fullUrl }));
				return null;
			} catch (e) {}
		}
		return origOpen.apply(this, arguments);
	};

	document.addEventListener('click', function(e) {
		const a = e.target.closest('a');
		if (!a) return;
		const target = (a.getAttribute('target') || '').toLowerCase();
		const href = a.href;
		if (href && (target === '_blank' || target === '_new' || e.button === 1 || e.ctrlKey || e.metaKey)) {
			e.preventDefault();
			e.stopPropagation();
			window.chrome.webview.postMessage(JSON.stringify({ type: 'open_tab', url: href }));
		}
	}, true);

	function notify() {
		if (document.title) {
			window.chrome.webview.postMessage(JSON.stringify({
				type: 'state',
				url: window.location.href,
				title: document.title
			}));
		}
	}
	window.addEventListener('DOMContentLoaded', notify);
	window.addEventListener('load', notify);
})();
`
