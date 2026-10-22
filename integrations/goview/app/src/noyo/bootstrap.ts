// Upstream chart modules create stores at module scope. Activate the shared
// Pinia before importing the canvas/router dependency tree.
import { setActivePinia } from 'pinia'
import { pinia } from '@/store'
setActivePinia(pinia)
