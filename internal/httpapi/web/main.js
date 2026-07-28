import './components/launcher-app.js'

document.addEventListener('contextmenu', (event) => {
  if (!event.target.closest('iframe')) {
    event.preventDefault()
  }
})
