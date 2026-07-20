import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'

import App from './App.vue'
import { createAppRouter } from './router'
import './styles/tailwind.css'
import './styles/main.scss'

const app = createApp(App)
app.use(createPinia())
app.use(createAppRouter())
app.use(ElementPlus)
app.mount('#app')
