#ifndef CAIRN_TRIP_RECORDER_H
#define CAIRN_TRIP_RECORDER_H

#include "../freematics-base/src/trip_types.h"
#include <cstddef>

// ---------------------------------------------------------------------------
// Ring buffer — fixed-capacity FIFO for buffering samples before flush
// ---------------------------------------------------------------------------
template <typename T, size_t CAPACITY>
class RingBuffer {
public:
    RingBuffer() : _head(0), _tail(0), _count(0) {}

    bool push(const T& item) {
        if (_count >= CAPACITY) return false;
        _buf[_head] = item;
        _head = (_head + 1) % CAPACITY;
        ++_count;
        return true;
    }

    bool pop(T& out) {
        if (_count == 0) return false;
        out = _buf[_tail];
        _tail = (_tail + 1) % CAPACITY;
        --_count;
        return true;
    }

    const T& peek() const { return _buf[_tail]; }
    size_t   count() const { return _count; }
    size_t   capacity() const { return CAPACITY; }
    bool     isEmpty() const { return _count == 0; }
    bool     isFull() const { return _count >= CAPACITY; }
    void     clear() { _head = _tail = _count = 0; }

    // Drain all items into a flat array; returns the number copied.
    size_t drainTo(T* dest, size_t maxItems) {
        size_t copied = 0;
        while (copied < maxItems && _count > 0) {
            dest[copied++] = _buf[_tail];
            _tail = (_tail + 1) % CAPACITY;
            --_count;
        }
        return copied;
    }

private:
    T      _buf[CAPACITY];
    size_t _head;
    size_t _tail;
    size_t _count;
};

// ---------------------------------------------------------------------------
// Trip recorder state
// ---------------------------------------------------------------------------
enum class TripRecorderState : uint8_t {
    INACTIVE,
    RECORDING,
    FINALIZING,
};

// ---------------------------------------------------------------------------
// TripRecorder — manages trip lifecycle, sample buffering, and file output
// ---------------------------------------------------------------------------
class TripRecorder {
public:
    TripRecorder();

    // Trip lifecycle
    bool begin(const char* deviceId, const char* firmwareVersion);
    void end();

    // Sample ingestion
    bool addGNSSSample(const GNSSSample& sample);
    bool addIMUSummary(const IMUSummary& summary);
    bool addIMURawWindow(const IMURawSample* window, size_t count);
    bool addEvent(TripEvent::EventType type, int32_t lat, int32_t lon,
                  const char* details);

    // File I/O
    bool flush();
    bool finalize();

    // Accessors
    bool     isActive() const;
    uint32_t getSampleCount() const;
    uint32_t getIMUSummaryCount() const;
    uint32_t getElapsedMs() const;
    const char* getTripId() const;
    TripRecorderState getState() const;

    // ID generation — timestamp hex + random bytes
    static void generateTripId(char* out, size_t outLen);

private:
    static constexpr size_t GNSS_BUF_SIZE = 64;
    static constexpr size_t IMU_BUF_SIZE  = 32;
    static constexpr size_t MAX_EVENTS    = 64;

    TripRecorderState _state;
    char              _tripId[28];
    char              _deviceId[32];
    char              _firmwareVersion[16];

    uint64_t _startTimeMs;
    uint32_t _totalGNSSSamples;
    uint32_t _totalIMUSummaries;
    uint16_t _totalIMURawWindows;

    RingBuffer<GNSSSample, GNSS_BUF_SIZE> _gnssBuf;
    RingBuffer<IMUSummary, IMU_BUF_SIZE>  _imuBuf;

    TripEvent _events[MAX_EVENTS];
    size_t    _eventCount;

    bool _gnssFileOpen;
    bool _imuFileOpen;

    // Internal helpers
    bool writeGNSSBuffer();
    bool writeIMUBuffer();
    bool writeManifest();
    bool writeEvents();
    bool writeChecksums();
    void buildFilePath(const char* filename, char* out, size_t outLen) const;
};

#endif // CAIRN_TRIP_RECORDER_H
