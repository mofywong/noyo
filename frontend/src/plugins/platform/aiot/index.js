import ConnectionStatus from './components/ConnectionStatus.vue';

export default {
    name: 'AIoT',
    type: 'platform',
    topology: {
        protocol: 'MQTT'
    },
    components: {
        status: ConnectionStatus
    }
}
