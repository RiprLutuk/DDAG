import { createApp } from 'vue'
import App from '../app.vue'
import router from './router'
import '../assets/css/main.css'
import '../assets/css/overview-premium.css'

import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import StatCard from '../components/StatCard.vue'
import StatusBadge from '../components/StatusBadge.vue'
import UiModal from '../components/UiModal.vue'
import UiTable from '../components/UiTable.vue'

const app = createApp(App)
app.component('Icon', Icon)
app.component('PageHeader', PageHeader)
app.component('StatCard', StatCard)
app.component('StatusBadge', StatusBadge)
app.component('UiModal', UiModal)
app.component('UiTable', UiTable)
app.use(router).mount('#app')
