/*!
 * AISplash —— AI Arsenal 启动叙事动画组件(框架无关)
 * 依赖:gsap(需在本脚本之前加载 window.gsap)
 *
 *   const splash = AISplash.create({
 *     frames: [...8张图],      // 默认 SPLASH 分镜 8 帧
 *     size: '72vmin',          // 舞台尺寸
 *     title: '',               // 可选结尾标题(默认空 = 不显示)
 *     onComplete(){ ... },     // 播完(定格在"发射完成")回调
 *   });
 *   splash.play();    // 全屏播放启动叙事,结束定格
 *   splash.hide();    // 淡出整个 splash(应用就绪、要露主界面时调用)
 *   splash.replay();  // 重播
 *   splash.destroy();
 */
(function () {
  'use strict';

  var CSS = [
    '.ais-splash{position:fixed;inset:0;display:none;align-items:center;justify-content:center;z-index:9999;',
    'background:radial-gradient(900px 600px at 50% 48%,#0a0e1c 0%,#04050a 70%);overflow:hidden}',
    '.ais-splash::before{content:"";position:absolute;inset:0;pointer-events:none;opacity:.5;',
    'background-image:radial-gradient(1px 1px at 20% 30%,#fff6,transparent),radial-gradient(1px 1px at 70% 60%,#fff4,transparent),',
    'radial-gradient(1px 1px at 40% 80%,#fff5,transparent),radial-gradient(1px 1px at 85% 25%,#fff3,transparent),',
    'radial-gradient(1px 1px at 55% 15%,#fff4,transparent)}',
    '.ais-stage{position:relative;display:grid;place-items:center}',
    '.ais-glow{position:absolute;inset:-15%;border-radius:50%;pointer-events:none;opacity:0;',
    'background:radial-gradient(closest-side,rgba(124,92,255,.55),rgba(33,212,253,.25) 45%,transparent 72%);filter:blur(6px);transform:scale(.7)}',
    '.ais-ringping{position:absolute;inset:18%;border-radius:50%;border:2px solid rgba(255,214,107,0);pointer-events:none}',
    '.ais-flash{position:absolute;inset:0;border-radius:50%;background:#fff;opacity:0;pointer-events:none;mix-blend-mode:screen}',
    '.ais-frame{position:absolute;inset:0;width:100%;height:100%;object-fit:contain;opacity:0;will-change:transform,opacity;',
    'filter:drop-shadow(0 24px 50px rgba(0,0,0,.6))}',
    '.ais-particles{position:absolute;inset:0;pointer-events:none}',
    '.ais-p{position:absolute;left:50%;top:50%;width:6px;height:6px;border-radius:50%;background:#fff;box-shadow:0 0 8px #ffd66b;opacity:0}',
    '.ais-titlewrap{position:absolute;left:0;right:0;bottom:14%;text-align:center;opacity:0;pointer-events:none}',
    '.ais-title{font-size:clamp(1.8rem,5vw,3rem);font-weight:800;letter-spacing:.04em;',
    'background:linear-gradient(90deg,#7c5cff,#21d4fd);-webkit-background-clip:text;background-clip:text;color:transparent;',
    'font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",Segoe UI,system-ui,sans-serif}',
    '.ais-subtitle{color:#8b93a4;margin-top:.35rem;font-size:.95rem;letter-spacing:.18em}'
  ].join('');

  function injectCSS() {
    if (document.getElementById('aisplash-style')) return;
    var s = document.createElement('style'); s.id = 'aisplash-style'; s.textContent = CSS;
    document.head.appendChild(s);
  }

  var DEFAULT_FRAMES = [
    'SPLASH分镜 01-初始休眠.png', 'SPLASH分镜 02-眼睛微睁.png',
    'SPLASH分镜 03-眼睛发光.png', 'SPLASH分镜 04-核心激活.png',
    'SPLASH分镜 05-能量聚集.png', 'SPLASH分镜 06-光环展开.png',
    'SPLASH分镜 07-系统就绪.png', 'SPLASH分镜 08-发射完成.png'
  ];

  // 叙事节奏:[帧索引, 起始时间s, 光晕强度]
  var SEQ = [
    { i: 0, t: 0.00, g: 0.10 },
    { i: 1, t: 1.50, g: 0.18 },
    { i: 2, t: 2.15, g: 0.42 },
    { i: 3, t: 2.90, g: 0.60 },
    { i: 4, t: 3.70, g: 0.78 },
    { i: 5, t: 4.65, g: 0.95 },
    { i: 6, t: 5.80, g: 0.72 },
    { i: 7, t: 6.55, g: 1.00 }
  ];
  var FADE = 0.55;

  function create(opts) {
    if (!window.gsap) console.warn('[AISplash] 未检测到 window.gsap,动画将不可用。');
    var o = Object.assign({
      frames: DEFAULT_FRAMES, size: 'min(72vmin,600px)', title: '', subtitle: '', onComplete: null
    }, opts || {});
    injectCSS();

    var host = document.createElement('div');
    host.className = 'ais-splash';
    var framesHTML = o.frames.map(function (f, i) {
      return '<img class="ais-frame" data-i="' + i + '" src="' + f + '" alt="" />';
    }).join('');
    var titleHTML = (o.title || o.subtitle)
      ? '<div class="ais-titlewrap"><div class="ais-title">' + (o.title || '') + '</div>'
        + (o.subtitle ? '<div class="ais-subtitle">' + o.subtitle + '</div>' : '') + '</div>'
      : '';
    host.innerHTML =
      '<div class="ais-stage" style="width:' + o.size + ';height:' + o.size + '">' +
        '<div class="ais-glow"></div>' +
        '<div class="ais-ringping"></div>' +
        '<div class="ais-flash"></div>' +
        framesHTML +
        '<div class="ais-particles"></div>' +
      '</div>' + titleHTML;
    document.body.appendChild(host);

    var frames = Array.from(host.querySelectorAll('.ais-frame'));
    var glow = host.querySelector('.ais-glow');
    var flash = host.querySelector('.ais-flash');
    var ringping = host.querySelector('.ais-ringping');
    var particles = host.querySelector('.ais-particles');
    var titlewrap = host.querySelector('.ais-titlewrap');
    var reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    var tl = null;
    var idle = null;

    function reset() {
      gsap.set(frames, { opacity: 0, scale: 1 });
      gsap.set(frames[0], { opacity: 1 });
      gsap.set(glow, { opacity: 0, scale: 0.7 });
      gsap.set(flash, { opacity: 0 });
      gsap.set(ringping, { opacity: 0, scale: 0.8 });
      if (titlewrap) gsap.set(titlewrap, { opacity: 0, y: 14 });
      particles.innerHTML = '';
    }

    function spawnBurst() {
      var n = 26;
      for (var k = 0; k < n; k++) {
        var p = document.createElement('div'); p.className = 'ais-p';
        var tint = k % 3 === 0 ? '#7c5cff' : (k % 3 === 1 ? '#21d4fd' : '#ffd66b');
        p.style.background = tint; p.style.boxShadow = '0 0 8px ' + tint;
        particles.appendChild(p);
        var ang = (k / n) * Math.PI * 2 + Math.random() * 0.3;
        var dist = 120 + Math.random() * 180;
        gsap.fromTo(p, { opacity: 1, x: 0, y: 0, scale: 1 },
          { opacity: 0, x: Math.cos(ang) * dist, y: Math.sin(ang) * dist, scale: 0.2,
            duration: 0.9 + Math.random() * 0.5, ease: 'power2.out', delay: Math.random() * 0.12 });
      }
    }

    function play() {
      host.style.display = 'flex';
      if (tl) tl.kill();
      if (idle) idle.kill();
      reset();
      if (reduce) {
        gsap.set(frames, { opacity: 0 }); gsap.set(frames[frames.length - 1], { opacity: 1 });
        gsap.set(glow, { opacity: 0.8, scale: 1 });
        if (o.onComplete) o.onComplete();
        return;
      }
      tl = gsap.timeline({ onComplete: function () { if (o.onComplete) o.onComplete(); } });
      SEQ.forEach(function (s, idx) {
        if (idx > 0) {
          tl.to(frames[SEQ[idx - 1].i], { opacity: 0, duration: FADE, ease: 'power2.in' }, s.t);
          tl.fromTo(frames[s.i], { opacity: 0 }, { opacity: 1, duration: FADE, ease: 'power2.out' }, s.t);
        }
        tl.to(glow, { opacity: s.g, scale: 0.85 + s.g * 0.5, duration: FADE + 0.2, ease: 'power2.out' }, s.t);
        if (s.i === 2) tl.fromTo(glow, { opacity: s.g }, { opacity: s.g + 0.2, scale: 1.2, duration: 0.35, yoyo: true, repeat: 1, ease: 'sine.inOut' }, s.t + 0.15);
        if (s.i === 3) { tl.fromTo(flash, { opacity: 0 }, { opacity: 0.5, duration: 0.12, ease: 'power2.out' }, s.t + 0.05); tl.to(flash, { opacity: 0, duration: 0.4, ease: 'power2.in' }, s.t + 0.17); }
        if (s.i === 5) { tl.fromTo(frames[s.i], { scale: 1.0 }, { scale: 1.08, duration: 0.9, ease: 'power2.out' }, s.t); tl.fromTo(ringping, { opacity: 0, scale: 0.85 }, { opacity: 0.7, scale: 1.6, duration: 1.0, ease: 'power2.out' }, s.t + 0.1); tl.to(ringping, { opacity: 0, duration: 0.5 }, s.t + 0.9); }
        if (s.i === 6) { tl.to(frames[s.i], { scale: 1.0, duration: 0.6, ease: 'power2.out' }, s.t); if (titlewrap) tl.to(titlewrap, { opacity: 1, y: 0, duration: 0.7, ease: 'power3.out' }, s.t + 0.2); }
        if (s.i === 7) {
          tl.fromTo(flash, { opacity: 0 }, { opacity: 0.8, duration: 0.14, ease: 'power2.out' }, s.t + 0.02);
          tl.to(flash, { opacity: 0, duration: 0.6, ease: 'power2.in' }, s.t + 0.16);
          tl.add(spawnBurst, s.t + 0.05);
          tl.to(glow, { opacity: 1, scale: 1.35, duration: 0.7, ease: 'power2.out' }, s.t);
          tl.to(glow, { scale: 1.15, duration: 1.2, ease: 'sine.inOut' }, s.t + 0.7);
          if (titlewrap) tl.to(titlewrap, { opacity: 1, duration: 0.3 }, s.t);
        }
      });
      // 收尾待机呼吸
      idle = gsap.to(glow, { scale: '+=0.05', duration: 2, yoyo: true, repeat: -1, ease: 'sine.inOut' });
    }

    function hide(onDone) {
      if (idle) idle.kill();
      gsap.to(host, { opacity: 0, duration: 0.5, ease: 'power2.in', onComplete: function () { host.style.display = 'none'; gsap.set(host, { opacity: 1 }); if (onDone) onDone(); } });
    }

    function replay() { play(); }
    function destroy() { if (tl) tl.kill(); if (idle) idle.kill(); if (host.parentNode) host.parentNode.removeChild(host); }

    return { play: play, hide: hide, replay: replay, destroy: destroy, el: host };
  }

  window.AISplash = { create: create, DEFAULT_FRAMES: DEFAULT_FRAMES };
})();
