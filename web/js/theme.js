/* theme.js —— 主题（自动 / 亮 / 暗），放在 <head> 先跑，避免页面先白后黑闪一下。
 * 选择存在 localStorage 的 a4pt.theme。提供：App.theme。需要：无。 */
(function (App) {
  'use strict';
  const KEY = 'a4pt.theme';
  function get() { try { return localStorage.getItem(KEY) || 'auto'; } catch (e) { return 'auto'; } }
  function apply(v) {
    if (v === 'light' || v === 'dark') document.documentElement.setAttribute('data-theme', v);
    else document.documentElement.removeAttribute('data-theme');
    document.querySelectorAll('[data-theme-set]').forEach(b => b.classList.toggle('on', b.dataset.themeSet === v));
  }
  function set(v) { try { localStorage.setItem(KEY, v); } catch (e) { /* 不能保存也照样切换 */ } apply(v); }
  apply(get());
  document.addEventListener('DOMContentLoaded', () => {
    apply(get());
    document.querySelectorAll('[data-theme-set]').forEach(b => { b.onclick = () => set(b.dataset.themeSet); });
  });
  App.theme = { get, set };
})((window.App = window.App || {}));
