/*!
 * AILoading —— AI Arsenal 加载组件(框架无关)
 * 依赖:gsap(需在本脚本之前加载 window.gsap)
 *
 * 用法:
 *   const loading = AILoading.create({
 *     imageSrc: 'AI Arsenal 启动页 Loading.png',   // 加载图(必填,有默认)
 *     readyImageSrc: 'AI Arsenal 启动页 Ready.png', // 可选:ready() 时切换到的就绪图
 *     size: 230,                                    // 舞台像素尺寸
 *     startLabel: '加载中', readyLabel: '就绪',
 *     bar: { left,top,width,height, track, fill },  // 图内进度条几何/颜色(默认已按该图量好)
 *   });
 *   loading.show();
 *   loading.setProgress(0.45);            // 0..1,带动画 + 百分比同步
 *   loading.setLabel('正在拉取实例…');
 *   loading.ready({ autoHide: 700 });      // 冲到 100% + ping,可选自动收起
 *   loading.hide();                        // 强制收起
 *   loading.destroy();
 *
 * 注:原图不做任何修改。进度条是在 Loading 图上方精确叠加的动态填充。
 */
(function () {
  'use strict';

  var CSS = [
    '.ail-overlay{position:fixed;inset:0;display:none;align-items:center;justify-content:center;',
    'background:rgba(8,10,16,.55);backdrop-filter:blur(8px);-webkit-backdrop-filter:blur(8px);z-index:9999;}',
    '.ail-loader{position:relative;display:flex;flex-direction:column;align-items:center;gap:1.1rem}',
    '.ail-stage{position:relative;filter:drop-shadow(0 12px 24px rgba(0,0,0,.5))}',
    '.ail-glow{position:absolute;inset:-8%;border-radius:50%;pointer-events:none;opacity:0;',
    'background:radial-gradient(closest-side,rgba(124,92,255,.40),transparent 70%);filter:blur(4px)}',
    '.ail-ring{position:absolute;inset:0;border-radius:50%;border:2px solid rgba(167,255,220,.7);opacity:0;pointer-events:none}',
    '.ail-shot{position:absolute;inset:0;opacity:0;overflow:hidden;will-change:transform,opacity}',
    '.ail-shot-inner{position:absolute;inset:0;transform-origin:center center}',
    '.ail-frame{position:absolute;inset:0;width:100%;height:100%;object-fit:contain}',
    '.ail-inbar{position:absolute;border-radius:2px;overflow:hidden;opacity:0;z-index:2;pointer-events:none}',
    '.ail-infill{position:absolute;inset:0;width:100%;transform-origin:left center;transform:scaleX(0)}',
    '.ail-infill::after{content:"";position:absolute;top:0;left:-40%;width:40%;height:100%;',
    'background:linear-gradient(90deg,transparent,rgba(255,255,255,.55),transparent);animation:ailsweep 1.3s linear infinite}',
    '@keyframes ailsweep{0%{left:-40%}100%{left:110%}}',
    '.ail-label{font-size:1rem;font-weight:600;color:#f4f6fb;min-height:1.2em;',
    'font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",Segoe UI,system-ui,sans-serif}',
    '.ail-label .dots::after{content:"";animation:aildots 1.2s steps(4,end) infinite}',
    '@keyframes aildots{0%{content:""}25%{content:"."}50%{content:".."}75%{content:"..."}}',
    '.ail-pct{font-variant-numeric:tabular-nums;color:#8b93a4;font-size:.8rem;min-height:1em;',
    'font-family:-apple-system,BlinkMacSystemFont,system-ui,sans-serif}',
    '@media (prefers-reduced-motion: reduce){.ail-infill::after{animation:none}}'
  ].join('');

  function injectCSS() {
    if (document.getElementById('ailoading-style')) return;
    var s = document.createElement('style');
    s.id = 'ailoading-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }

  var DEFAULTS = {
    imageSrc: 'AI Arsenal 启动页 Loading.png',
    readyImageSrc: '',
    size: 360,
    startLabel: '加载中',
    readyLabel: '就绪',
    // 内容缩放:Loading/Ready 图是"角色居中 + 四周留白"(角色仅占画布 67/73%),
    // 放大裁掉留白让角色铺满舞台。数值已按这两张图量出;换图用 measure-image.py 重新量(≈ 100/内容宽度覆盖率)。
    contentScale: 1.29,
    readyContentScale: 1.18,
    // 进度条几何/颜色 —— 已按 Loading 图精确量出:原图是"两条细绿线之间的近黑轨道 + 绿填充",
    // 这里只接管"两线之间"的填充区,两条细线保留原图,故是同一条 bar 在涨。换图可覆盖。
    bar: {
      left: '20.12%', top: '85.8%', width: '59.67%', height: '3.0%',
      track: 'rgb(3,21,22)', fill: 'rgb(63,189,123)'
    }
  };

  function applyBar(barEl, b) {
    barEl.style.left = b.left;
    barEl.style.top = b.top;
    barEl.style.width = b.width;
    barEl.style.height = b.height;
    barEl.style.background = b.track;
    var fill = barEl.firstElementChild;
    fill.style.background = 'linear-gradient(180deg, rgba(255,255,255,.15), transparent 55%), ' + b.fill;
    fill.style.boxShadow = '0 0 6px ' + b.fill;
  }

  function create(opts) {
    if (!window.gsap) console.warn('[AILoading] 未检测到 window.gsap,动画将不可用。');
    var o = Object.assign({}, DEFAULTS, opts || {});
    o.bar = Object.assign({}, DEFAULTS.bar, (opts && opts.bar) || {});
    injectCSS();

    var overlay = document.createElement('div');
    overlay.className = 'ail-overlay';
    overlay.innerHTML =
      '<div class="ail-loader">' +
        '<div class="ail-stage" style="width:' + o.size + 'px;height:' + o.size + 'px">' +
          '<div class="ail-glow"></div>' +
          '<div class="ail-ring"></div>' +
          '<div class="ail-shot ail-shot-load">' +
            '<div class="ail-shot-inner" style="transform:scale(' + o.contentScale + ')">' +
              '<img class="ail-frame" src="' + o.imageSrc + '" alt="" />' +
              '<div class="ail-inbar"><div class="ail-infill"></div></div>' +
            '</div>' +
          '</div>' +
          (o.readyImageSrc
            ? '<div class="ail-shot ail-shot-ready"><div class="ail-shot-inner" style="transform:scale(' + o.readyContentScale + ')"><img class="ail-frame" src="' + o.readyImageSrc + '" alt="" /></div></div>'
            : '') +
        '</div>' +
        '<div class="ail-label"></div>' +
        '<div class="ail-pct"></div>' +
      '</div>';
    document.body.appendChild(overlay);

    var el = {
      overlay: overlay,
      stage: overlay.querySelector('.ail-stage'),
      load: overlay.querySelector('.ail-shot-load'),
      ready: overlay.querySelector('.ail-shot-ready'),
      glow: overlay.querySelector('.ail-glow'),
      ring: overlay.querySelector('.ail-ring'),
      bar: overlay.querySelector('.ail-inbar'),
      fill: overlay.querySelector('.ail-infill'),
      label: overlay.querySelector('.ail-label'),
      pct: overlay.querySelector('.ail-pct')
    };
    applyBar(el.bar, o.bar);

    var reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    var shown = false;
    var idle = null;
    var pctObj = { v: 0 };

    function setLabel(t, dots) {
      el.label.innerHTML = t + (dots ? '<span class="dots"></span>' : '');
    }

    function show(conf) {
      conf = conf || {};
      if (shown) return;
      shown = true;
      setLabel(conf.label != null ? conf.label : o.startLabel, true);
      el.overlay.style.display = 'flex';
      el.pct.textContent = '';
      pctObj.v = 0;
      gsap.killTweensOf([el.overlay, el.load, el.ready, el.glow, el.bar, el.fill, pctObj]);
      gsap.set([el.load, el.ready], { opacity: 0, scale: 0.9 });
      gsap.set(el.fill, { scaleX: 0 });
      gsap.set(el.glow, { opacity: 0, scale: 0.6 });
      gsap.set(el.bar, { opacity: 0 });

      if (reduce) {
        gsap.set(el.overlay, { opacity: 1 });
        gsap.set(el.load, { opacity: 1, scale: 1 });
        gsap.set(el.bar, { opacity: 1 });
        return;
      }
      var tl = gsap.timeline();
      gsap.set(el.overlay, { opacity: 0 });
      tl.to(el.overlay, { opacity: 1, duration: 0.2 });
      tl.fromTo(el.load, { opacity: 0, scale: 0.82 }, { opacity: 1, scale: 1, duration: 0.5, ease: 'back.out(1.8)' }, '<');
      tl.fromTo(el.glow, { opacity: 0, scale: 0.6 }, { opacity: 1, scale: 1, duration: 0.5, ease: 'power2.out' }, '<');
      tl.to(el.bar, { opacity: 1, duration: 0.3 }, '<+=0.1');
      idle && idle.kill();
      // 待机:整体上下轻浮(y 平移,不改进度条填充宽度,避免"进度回退"错觉)
      idle = gsap.to(el.stage, { y: -6, duration: 1.1, yoyo: true, repeat: -1, ease: 'sine.inOut' });
    }

    function setProgress(frac, conf) {
      if (!shown) show();
      frac = Math.max(0, Math.min(1, frac));
      conf = conf || {};
      if (conf.label != null) setLabel(conf.label, true);
      if (reduce) {
        el.fill.style.transform = 'scaleX(' + frac + ')';
        el.pct.textContent = Math.round(frac * 100) + '%';
        pctObj.v = frac * 100;
        return;
      }
      gsap.killTweensOf(el.fill);
      gsap.to(el.fill, { scaleX: frac, duration: 0.5, ease: 'power1.out' });
      gsap.killTweensOf(pctObj);
      gsap.to(pctObj, { v: frac * 100, duration: 0.5, ease: 'power1.out',
        onUpdate: function () { el.pct.textContent = Math.round(pctObj.v) + '%'; } });
    }

    function setLabel_(t, dots) { setLabel(t, dots); }

    function ready(conf) {
      conf = conf || {};
      if (!shown) show();
      setLabel(conf.label != null ? conf.label : o.readyLabel, false);
      if (reduce) {
        el.fill.style.transform = 'scaleX(1)';
        el.pct.textContent = '100%';
        if (conf.autoHide) setTimeout(hide, conf.autoHide);
        return;
      }
      gsap.killTweensOf([el.fill, el.glow, el.ring, el.load, el.ready, el.bar, pctObj]);
      idle && idle.kill(); gsap.set(el.stage, { y: 0 });
      gsap.to(el.fill, { scaleX: 1, duration: 0.3, ease: 'power2.out' });
      gsap.to(pctObj, { v: 100, duration: 0.3, onUpdate: function () { el.pct.textContent = Math.round(pctObj.v) + '%'; } });
      gsap.fromTo(el.ring, { opacity: 0.8, scale: 0.9 }, { opacity: 0, scale: 1.5, duration: 0.7, ease: 'power2.out' });
      gsap.fromTo(el.glow, { opacity: 0.6 }, { opacity: 1, duration: 0.3, yoyo: true, repeat: 1, ease: 'sine.inOut' });
      if (el.ready) {
        gsap.to(el.load, { opacity: 0, scale: 1.08, duration: 0.3, ease: 'power2.in' });
        gsap.fromTo(el.ready, { opacity: 0, scale: 0.85 }, { opacity: 1, scale: 1, duration: 0.45, ease: 'back.out(2)' });
        gsap.to(el.bar, { opacity: 0, duration: 0.25 });
      }
      if (conf.autoHide) gsap.delayedCall(conf.autoHide / 1000, hide);
    }

    function hide() {
      if (!shown) return;
      shown = false;
      idle && idle.kill(); gsap.set(el.stage, { y: 0 });
      if (reduce) { el.overlay.style.display = 'none'; return; }
      gsap.killTweensOf(el.overlay);
      gsap.to(el.overlay, {
        opacity: 0, duration: 0.25, ease: 'power2.in',
        onComplete: function () {
          el.overlay.style.display = 'none';
          gsap.set([el.load, el.ready], { opacity: 0, scale: 0.9 });
        }
      });
    }

    function destroy() {
      hide();
      window.setTimeout(function () { if (overlay.parentNode) overlay.parentNode.removeChild(overlay); }, 400);
    }

    return {
      show: show,
      setProgress: setProgress,
      setLabel: setLabel_,
      ready: ready,
      hide: hide,
      destroy: destroy,
      el: el
    };
  }

  window.AILoading = { create: create, DEFAULTS: DEFAULTS };
})();
