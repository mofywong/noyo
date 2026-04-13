import ConnectionStatus from './components/ConnectionStatus.vue';

export default {
    name: 'Sagoo',
    type: 'platform',
    topology: {
        protocol: 'MQTT'
    },
    components: {
        status: ConnectionStatus
    }
}
