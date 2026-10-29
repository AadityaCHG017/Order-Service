UPDATE orders
SET status = 'pending'
WHERE status = 'cancelling';

ALTER TABLE orders
DROP CONSTRAINT orders_status_check;

ALTER TABLE orders
ADD CONSTRAINT orders_status_check
CHECK (status IN (
    'pending',
    'confirmed',
    'cancelled',
    'completed'
));
