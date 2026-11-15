/* 第六轮：切换动效。
   设计稿每次状态变化都会整体重建 DOM（render），所以「滑动」靠 FLIP：渲染前记下每个容器里选中项的位置，
   渲染后把新的指示条从旧位置过渡到新位置；同一页面里切子视图时，给新内容播 view-up。
   应用（Go）里的做法是同一个思路：MyGo 的 Transition{Match} 让新出现的指示条从同一标识的旧位置过来。 */
(function () {
  // 哪些容器有「选中项」：sel 选容器，item 选容器里的项，kind 决定指示条的几何
  const HOSTS = [
    { sel: '.seg', item: ':scope > button', kind: 'box' },
    { sel: '.vseg', item: ':scope > button', kind: 'box' },
    { sel: '.tabs', item: ':scope > button', kind: 'line' },
    { sel: '.A .rail', item: ':scope > .rbtn', kind: 'box' },
  ];
  const last = {}; // key → { l, t, w, h, idx, page }

  const speed = () => { const m = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--m')); return isNaN(m) ? 1 : m; };
  const DUR = 200, EASE = 'cubic-bezier(.2,.8,.2,1)';

  // 内容区：容器后面紧跟的那一块（容器自己的下一个兄弟，或它父元素的下一个兄弟）
  function regionOf(host) {
    for (const n of [host.nextElementSibling, host.parentElement && host.parentElement.nextElementSibling]) {
      if (n && n.offsetHeight > host.offsetHeight && !n.matches('.slider')) return n;
    }
    return null;
  }

  function enhance(page) {
    const root = $('#winmount'); if (!root) return;
    const m = speed();
    HOSTS.forEach(h => {
      $$(h.sel, root).forEach((host, n) => {
        const items = $$(h.item, host), idx = items.findIndex(b => b.classList.contains('on'));
        // 页面内容里的容器换页就是新容器（key 带页面名，不滑动）；窗口外壳里的（活动栏、视图切换）跨页也要滑，key 不带页面
        const key = `${host.closest('#pg') ? page : ''}|${h.sel}|${n}`, prev = last[key];
        if (idx < 0) { delete last[key]; return; }
        const it = items[idx];
        host.classList.add('sl-host');
        const r = h.kind === 'line'
          ? { l: it.offsetLeft, t: it.offsetTop + it.offsetHeight - 1, w: it.offsetWidth, h: 2 }
          : { l: it.offsetLeft, t: it.offsetTop, w: it.offsetWidth, h: it.offsetHeight };
        const s = document.createElement('i'); s.className = 'slider';
        Object.assign(s.style, { left: r.l + 'px', top: r.t + 'px', width: r.w + 'px', height: r.h + 'px' });
        host.prepend(s);
        if (prev && m > 0 && (prev.l !== r.l || prev.t !== r.t || prev.w !== r.w || prev.h !== r.h)) {
          s.animate([
            { transform: `translate(${prev.l - r.l}px, ${prev.t - r.t}px)`, width: prev.w + 'px', height: prev.h + 'px' },
            { transform: 'translate(0, 0)', width: r.w + 'px', height: r.h + 'px' },
          ], { duration: DUR * m, easing: EASE });
        }
        // 选中项变了：子视图的内容淡入（整页切换已经有 page-in，所以只在页面没变时播）
        if (prev && prev.idx !== idx && prev.page === page && m > 0) {
          const reg = regionOf(host);
          if (reg) reg.animate([{ opacity: 0, transform: 'translateY(6px)' }, { opacity: 1, transform: 'none' }], { duration: DUR * m, easing: EASE });
        }
        last[key] = { ...r, idx, page };
      });
    });
  }

  // 包住 render：渲染完成后挂指示条。page 取当前页面，切页时不做子视图淡入
  const orig = window.render;
  window.render = function (anim) {
    // 重绘前的状态：侧栏是否已经展开、当前在哪一页（用来判断这次重绘是不是「同一页里的小变化」）
    const hadSide = !!document.querySelector('#winmount .side:not(.closing)');
    const before = S.page + '|' + (S.xtab || '');
    const out = orig.apply(this, arguments);
    try {
      const page = S.page + '|' + (S.xtab || '');
      if (!PAGES[S.page] || PAGES[S.page].group !== 'sys') {
        // DOM 是整体重建的，入场动画会跟着重放：侧栏本来就在就不要再从 0 宽度滑出来，同页重绘列表也不要再逐行入场
        if (hadSide) { const sd = $('#winmount .side'); if (sd) sd.classList.add('still'); }
        if (before === page) $$('#winmount .stagger > *').forEach(e => { e.style.animation = 'none'; });
        enhance(page);
      }
    } catch (e) { console.warn('motion6', e); }
    return out;
  };
})();
