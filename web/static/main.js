const ready = (function () {
  if (document.readyState === 'complete') {
    return Promise.resolve();
  }
  return new Promise(function (resolve) {
    window.addEventListener('load', resolve);
  });
}());

ready.then(()=> {
  const quitBtn = document.getElementById('quit');
  quitBtn.addEventListener('click', () => quit().then(Promise.resolve))
})

