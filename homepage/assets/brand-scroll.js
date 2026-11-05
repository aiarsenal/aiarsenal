(async () => {
  const container = document.querySelector('.hero-mark');
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  if (reduce.matches) return;
  const response = await fetch(container.dataset.motion);
  if (!response.ok) return;
  const documentSvg = new DOMParser().parseFromString(await response.text(), 'image/svg+xml');
  if (documentSvg.documentElement.localName !== 'svg') return;
  const mark = document.importNode(documentSvg.documentElement, true);
  mark.id = 'hero-eyebrow';
  mark.setAttribute('aria-label', 'AI Arsenal');
  container.replaceChildren(mark);
  const heading = document.querySelector('.hero-copy h1');
  const nav = document.querySelector('body > nav');
  const playhead = { progress: 0 };
  const riseStart = 0.9;
  const riseDuration = 0.86;
  let target = 0;
  let scheduled = false;

  // Reuse the authored rise from the SVG. Its own keyframes shape the movement.
  mark.pauseAnimations();
  function draw() {
    mark.setCurrentTime(riseStart + riseDuration * playhead.progress);
  }

  function update() {
    scheduled = false;
    const offset = Math.max(0, window.scrollY);
    const fullRaiseAt = Math.max(1,
      heading.getBoundingClientRect().top + offset - nav.offsetHeight - 12,
    );
    const atTop = offset <= 0.5;
    const next = reduce.matches || atTop ? 0 : Math.max(target, Math.min(1, offset / fullRaiseAt));
    if (reduce.matches) {
      gsap.killTweensOf(playhead);
      target = 0;
      playhead.progress = 0;
      draw();
      return;
    }
    if (next === target) return;
    target = next;
    gsap.to(playhead, {
      progress: target,
      duration: atTop ? 0.62 : 0.18,
      ease: atTop ? 'sine.inOut' : 'sine.out',
      overwrite: true,
      onUpdate: draw,
    });
  }

  function schedule() {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(update);
  }

  draw();
  update();
  addEventListener('scroll', schedule, { passive: true });
  addEventListener('resize', schedule);
  reduce.addEventListener('change', schedule);
})().catch(error => console.warn("Brand animation unavailable", error));
