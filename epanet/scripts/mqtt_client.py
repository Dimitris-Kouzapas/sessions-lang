import json
import queue
import threading

import paho.mqtt.client as mqtt

class MQTTClient:
    def __init__(self, client_id = "epyt-simulator", broker = "127.0.0.1", port = 1883):
        self.subscriptions = {}

        # Protects self.subscriptions itself.
        self._lock = threading.Lock()

        self.broker = broker
        self.port = port

        self.client = mqtt.Client(
            callback_api_version=mqtt.CallbackAPIVersion.VERSION2,
            client_id=client_id,
            protocol=mqtt.MQTTv311,
        )

        self.client.on_connect = self._on_connect
        self.client.on_message = self._on_message

        self.client.reconnect_delay_set(
            min_delay=1,
            max_delay=5,
        )

    def start(self):
        self.client.connect(
            self.broker,
            self.port,
            keepalive=60,
        )

        self.client.loop_start()

    def stop(self):
        self.client.disconnect()
        self.client.loop_stop()

    def _on_connect(
        self,
        client,
        userdata,
        flags,
        reason_code,
        properties,
    ):
        if reason_code.is_failure:
            print(f"MQTT connection failed: {reason_code}")
            return

        print("Connected to MQTT broker")

        # Copy subscriptions while holding the lock.
        with self._lock:
            subscriptions = [
                (topic, info["qos"])
                for topic, info in self.subscriptions.items()
            ]

        # Don't hold our lock while calling Paho.
        for topic, qos in subscriptions:
            client.subscribe(topic, qos)

    def _on_message(
        self,
        client,
        userdata,
        message,
    ):
        try:
            value = json.loads(
                message.payload.decode("utf-8")
            )
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            print(f"Invalid message: {exc} | ", message.payload)
            return

        # Safely look up the subscription.
        with self._lock:
            subscription = self.subscriptions.get(message.topic)

        if subscription is not None:
            # Queue itself is thread-safe.
            subscription["queue"].put(value)

    def subscribe(self, topic, qos=0):
        with self._lock:
            if topic in self.subscriptions:
                return

            self.subscriptions[topic] = {
                "qos": qos,
                "queue": queue.Queue(),
            }

        self.client.subscribe(topic, qos)

    def get(self, topic, block=False, timeout=None):
        with self._lock:
            subscription = self.subscriptions.get(topic)

        if subscription is None:
            return None

        try:
            return subscription["queue"].get(
                block=block,
                timeout=timeout,
            )
        except queue.Empty:
            return None

    def publish(self, topic, value, qos=0, retain=False):
        payload = json.dumps(
            value,
            separators=(",", ":"),
        )

        return self.client.publish(
            topic,
            payload,
            qos=qos,
            retain=retain,
        )
