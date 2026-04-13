import ConnectionStatus from './components/ConnectionStatus.vue';

export default {
    name: 'MQTT_API',
    type: 'platform',
    topology: {
        protocol: 'MQTT'
    },
    components: {
        status: ConnectionStatus
    }
}
