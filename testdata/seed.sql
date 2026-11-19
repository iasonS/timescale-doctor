-- Demo database for tsdoctor: a few hypertables with the mistakes the report looks for.
-- Times are relative to now(), so the report reads the same whenever it runs.

\set ON_ERROR_STOP on
set client_min_messages = warning;

create extension if not exists timescaledb;
create extension if not exists pg_stat_statements;

-- Weather readings, compressed and with a retention policy, but both policies are paused,
-- so old chunks are neither dropped nor compressed.
create table conditions (
    time        timestamptz not null,
    city        text not null,
    temperature double precision,
    humidity    double precision
);
select create_hypertable('conditions', 'time', chunk_time_interval => interval '1 day');

insert into conditions
select t, city, 10 + 15 * random(), 40 + 50 * random()
from generate_series(now() - interval '60 days', now(), interval '1 minute') as t,
     unnest(array['Lisbon', 'Oslo', 'Cairo', 'Lima']) as city;

alter table conditions set (timescaledb.compress, timescaledb.compress_segmentby = 'city');
select count(compress_chunk(c)) as compressed from show_chunks('conditions', older_than => interval '20 days') as c;

-- In one transaction, so the scheduler never sees the policies before they are paused.
begin;
select add_compression_policy('conditions', interval '7 days');
select add_retention_policy('conditions', interval '30 days');
select alter_job(job_id, scheduled => false)
from timescaledb_information.jobs
where hypertable_name = 'conditions';
commit;

-- Web traffic with one-hour chunks and no retention: over a thousand chunks.
create table page_views (
    time        timestamptz not null,
    path        text not null,
    duration_ms integer
);
select create_hypertable('page_views', 'time', chunk_time_interval => interval '1 hour');

insert into page_views
select t, path, (random() * 900)::integer
from generate_series(now() - interval '50 days', now(), interval '10 minutes') as t,
     unnest(array['/', '/pricing', '/docs']) as path;

-- API requests: compressed by hand, but nothing compresses new chunks.
create table api_requests (
    time       timestamptz not null,
    endpoint   text not null,
    status     integer,
    latency_ms double precision
);
select create_hypertable('api_requests', 'time', chunk_time_interval => interval '1 day');

insert into api_requests
select t, endpoint, (array[200, 200, 200, 404, 500])[1 + (random() * 4)::integer], random() * 250
from generate_series(now() - interval '14 days', now(), interval '1 minute') as t,
     unnest(array['/orders', '/users']) as endpoint;

alter table api_requests set (timescaledb.compress, timescaledb.compress_segmentby = 'endpoint');
select count(compress_chunk(c)) as compressed from show_chunks('api_requests', older_than => interval '3 days') as c;
select add_retention_policy('api_requests', interval '90 days');

-- A hypertable on an integer column instead of a time.
create table counters (
    tick  bigint not null,
    value double precision
);
select create_hypertable('counters', 'tick', chunk_time_interval => 100000);

insert into counters
select tick, random()
from generate_series(1, 500000) as tick;

-- Hourly averages, refreshed by a policy.
create materialized view conditions_hourly
with (timescaledb.continuous, timescaledb.materialized_only = true) as
select time_bucket('1 hour', time) as bucket, city, avg(temperature) as avg_temperature
from conditions
group by bucket, city
with no data;

select add_continuous_aggregate_policy('conditions_hourly',
    start_offset => interval '3 days',
    end_offset => interval '1 hour',
    schedule_interval => interval '1 hour');

-- Daily maximums with real-time aggregation, refreshed once by hand and never again.
create materialized view conditions_daily
with (timescaledb.continuous, timescaledb.materialized_only = false) as
select time_bucket('1 day', time) as bucket, city, max(temperature) as max_temperature
from conditions
group by bucket, city
with no data;

call refresh_continuous_aggregate('conditions_daily', null, now() - interval '20 days');

-- A user-defined job that always fails.
create procedure export_totals(job_id integer, config jsonb)
language plpgsql
as $$
begin
    raise exception 'table export_target does not exist';
end
$$;

select add_job('export_totals', interval '1 minute');

-- A few statements for pg_stat_statements to show.
select city, avg(temperature) from conditions where time > now() - interval '1 day' group by city;
select path, count(*) from page_views where time > now() - interval '7 days' group by path;
select endpoint, percentile_cont(0.99) within group (order by latency_ms) from api_requests group by endpoint;
