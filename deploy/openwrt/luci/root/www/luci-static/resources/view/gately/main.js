'use strict';
'require view';
'require dom';
'require form';
'require poll';
'require rpc';
'require ui';

// LuCI「服务 → Gately」页面：显示运行状态，提供启停、开机自启与打开管理界面，并可在此覆盖管理界面端口。
// 状态与操作来自 rpcd 插件 /usr/libexec/rpcd/luci.gately，端口保存在 /etc/config/gately，由启动脚本传给 gately -listen。

var DEFAULT_PORT = '16680';

var callStatus = rpc.declare({
	object: 'luci.gately',
	method: 'status',
	expect: { '': {} }
});

var callAction = rpc.declare({
	object: 'luci.gately',
	method: 'action',
	params: [ 'name' ],
	expect: { '': {} }
});

// 拆分监听地址：":16680" → { host: '', port: '16680' }，"[::1]:16680" → { host: '::1', port: '16680' }
function splitListen(addr) {
	var m = String(addr || '').trim().match(/^(?:\[([^\]]*)\]|([^:]*)):(\d+)$/);

	if (!m) {
		return null;
	}
	return { host: m[1] != null ? m[1] : (m[2] || ''), port: m[3] };
}

function isWildcard(host) {
	return host === '' || host === '0.0.0.0' || host === '::';
}

function isLoopback(host) {
	return host === 'localhost' || host === '::1' || /^127\./.test(host);
}

// 与 Gately 保存全局设置时的规范化一致：补前导 /，去掉末尾 /
function normalizeSafePath(path) {
	path = String(path || '').trim();
	if (path === '') {
		return '';
	}
	if (path.charAt(0) !== '/') {
		path = '/' + path;
	}
	return path.replace(/\/+$/, '');
}

// 生成浏览器可访问的管理界面地址。监听在全部地址上时沿用当前 LuCI 页面的主机名
function adminURL(scheme, listen, safePath) {
	var host = isWildcard(listen.host) ? window.location.hostname : listen.host;

	if (host.indexOf(':') >= 0 && host.charAt(0) !== '[') {
		host = '[' + host + ']';
	}
	return scheme + '://' + host + ':' + listen.port + (normalizeSafePath(safePath) || '/');
}

function validPort(v) {
	return /^\d+$/.test(v || '') && +v >= 1 && +v <= 65535;
}

// 实际生效的 HTTP 监听地址：gately.json 的 listen，LuCI 中设置了端口时只替换端口、主机部分不变（与启动脚本一致），
// 都没有时为默认端口
function effectiveListen(st) {
	var base = splitListen(st.listen) || { host: '', port: DEFAULT_PORT };

	if (validPort(st.port)) {
		return { host: base.host, port: st.port };
	}
	return base;
}

function formatMemory(kb) {
	if (kb >= 1024) {
		return '%.1f MB'.format(kb / 1024);
	}
	return '%d KB'.format(kb);
}

return view.extend({
	load: function() {
		return callStatus();
	},

	statusRow: function(label, value) {
		return E('tr', { 'class': 'tr' }, [
			E('td', { 'class': 'td left', 'width': '33%' }, label),
			E('td', { 'class': 'td left' }, value)
		]);
	},

	renderAddresses: function(st) {
		var http = effectiveListen(st),
		    https = splitListen(st.https_listen),
		    links = [],
		    notes = [];

		// LuCI 的 E() 把单个字符串子节点当作 HTML 插入，地址与说明来自 Gately 配置，包成数组按纯文本插入
		links.push(E('a', { 'href': adminURL('http', http, st.safe_path), 'target': '_blank', 'rel': 'noreferrer' },
			[ adminURL('http', http, st.safe_path) ]));
		if (https) {
			links.push(E('br'));
			links.push(E('a', { 'href': adminURL('https', https, st.safe_path), 'target': '_blank', 'rel': 'noreferrer' },
				[ adminURL('https', https, st.safe_path) ]));
		}
		if (isLoopback(http.host)) {
			notes.push(_('Only reachable from the router itself (listening on %s)').format(http.host + ':' + http.port));
		}
		if (st.lan_only) {
			notes.push(_('LAN only is on in the Gately global settings, so only LAN addresses can open it'));
		}
		notes.forEach(function(note) {
			links.push(E('div', { 'class': 'cbi-value-description' }, [ note ]));
		});
		return links;
	},

	renderStatus: function(st) {
		var running = !!st.running,
		    rows = [];

		if (!st.installed) {
			return E('p', { 'class': 'alert-message warning' },
				_('Gately is not installed. Install the gately package first.'));
		}

		rows.push(this.statusRow(_('Running state'), running
			? E('span', { 'style': 'color:green;font-weight:bold' }, _('Running (PID %d)').format(st.pid))
			: E('span', { 'style': 'color:red;font-weight:bold' }, _('Not running'))));
		rows.push(this.statusRow(_('Version'), [ st.version || '-' ]));
		if (running && st.rss_kb) {
			rows.push(this.statusRow(_('Memory'), formatMemory(st.rss_kb)));
		}
		rows.push(this.statusRow(_('Start on boot'), st.enabled ? _('Enabled') : _('Disabled')));
		rows.push(this.statusRow(_('Admin interface'), this.renderAddresses(st)));
		rows.push(this.statusRow(_('Configuration directory'), [ st.conf_dir ]));

		return E('table', { 'class': 'table' }, rows);
	},

	renderButtons: function(st) {
		var self = this,
		    buttons = [];

		if (!st.installed) {
			return E('div');
		}

		buttons.push(E('button', {
			'class': 'btn cbi-button cbi-button-action',
			'disabled': st.running ? null : true,
			'click': function(ev) {
				ev.preventDefault();
				window.open(adminURL('http', effectiveListen(st), st.safe_path), '_blank', 'noreferrer');
			}
		}, _('Open admin interface')));

		var add = function(name, label, cls) {
			buttons.push(' ');
			buttons.push(E('button', {
				'class': 'btn cbi-button ' + cls,
				'click': ui.createHandlerFn(self, 'handleServiceAction', name)
			}, label));
		};

		if (st.running) {
			add('stop', _('Stop'), 'cbi-button-reset');
			add('restart', _('Restart'), 'cbi-button-apply');
		} else {
			add('start', _('Start'), 'cbi-button-apply');
		}
		if (st.enabled) {
			add('disable', _('Disable start on boot'), 'cbi-button-neutral');
		} else {
			add('enable', _('Enable start on boot'), 'cbi-button-neutral');
		}

		// 不使用 cbi-page-actions 类，避免与页面底部的「保存并应用」按钮区混淆
		return E('div', { 'style': 'margin:1em 0' }, buttons);
	},

	// 刷新状态区：首次渲染与轮询共用
	updateStatus: function(st) {
		var node = document.getElementById('gately-status');

		if (!node) {
			return;
		}
		dom.content(node, [ this.renderStatus(st), this.renderButtons(st) ]);
	},

	handleServiceAction: function(name) {
		var self = this;

		return callAction(name).then(function(res) {
			if (res.error || res.code) {
				ui.addNotification(null, E('p', [ _('Failed to run the action: %s').format(res.error || ('exit code ' + res.code)) ]), 'error');
			}
		}).catch(function(err) {
			ui.addNotification(null, E('p', [ _('Failed to run the action: %s').format(err.message || err) ]), 'error');
		}).then(function() {
			// procd 启停是异步的，稍等再读状态
			return new Promise(function(resolve) {
				window.setTimeout(resolve, 1000);
			});
		}).then(function() {
			return callStatus();
		}).then(function(st) {
			self.updateStatus(st);
		});
	},

	render: function(st) {
		var self = this,
		    m, s, o, current;

		m = new form.Map('gately', _('Gately'),
			_('Gately is an open-source network gateway: port forwarding, reverse proxy, DDNS, ACME certificates, WAF and more. Manage the service here and open the Gately admin interface for everything else.'));

		s = m.section(form.NamedSection, 'main', 'gately', _('Settings'));
		s.anonymous = true;
		s.addremove = false;

		current = splitListen(st.listen) || { host: '', port: DEFAULT_PORT };
		o = s.option(form.Value, 'port', _('Admin port'),
			_('Leave empty to use the listen address in the Gately global settings (currently %s). When set, Gately listens on this port and keeps the host part of that address; listen address changes made in Gately are saved and take effect after this field is cleared.')
				.format('%h'.format((current.host.indexOf(':') >= 0 ? '[' + current.host + ']' : current.host) + ':' + current.port)));
		o.datatype = 'range(1,65535)';
		o.placeholder = current.port;
		o.rmempty = true;
		// 与 HTTPS 监听同端口时 HTTP 会先占住端口，HTTPS 起不来；Gately 保存设置时的同端口检查在这里被绕过，需要单独拦截
		o.validate = function(section_id, value) {
			var https = splitListen(st.https_listen);

			if (value && https && https.port === String(value).trim()) {
				return _('This port is already used by the HTTPS listen address in the Gately global settings');
			}
			return true;
		};

		poll.add(function() {
			return callStatus().then(function(res) {
				self.updateStatus(res);
			});
		}, 5);

		return m.render().then(function(mapNode) {
			var statusSection = E('div', { 'class': 'cbi-section' }, [
				E('h3', _('Status')),
				E('div', { 'id': 'gately-status' }, [ self.renderStatus(st), self.renderButtons(st) ]),
				E('div', { 'class': 'cbi-section-descr' },
					_('Default account admin / gately.cn. If you forget the password, run gately reset-password on the router.'))
			]);

			// 状态区放在页面说明之后、设置区之前
			var firstSection = mapNode.querySelector('.cbi-section');

			if (firstSection) {
				firstSection.parentNode.insertBefore(statusSection, firstSection);
			} else {
				mapNode.appendChild(statusSection);
			}
			return mapNode;
		});
	}
});
