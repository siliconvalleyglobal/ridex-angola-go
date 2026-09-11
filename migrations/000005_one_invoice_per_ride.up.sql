CREATE UNIQUE INDEX idx_saft_invoices_one_per_ride
    ON saft_invoices(ride_id);
