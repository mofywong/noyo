import ConnectionStatus from '@/plugins/platform/mqtt_api/components/ConnectionStatus.vue';
import ConfigPanel from './components/ConfigPanel.vue';

export default {
    name: 'cascade',
    type: 'platform',
    components: {
        status: ConnectionStatus,
        config: ConfigPanel
    },
    topology: {
        protocol: 'MQTT'
    }
}
