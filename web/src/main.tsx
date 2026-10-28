import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@want-foundation/desyst-foundation/styles.css'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
