-- +goose Up
ALTER TABLE person_bonds DROP CONSTRAINT person_bonds_kind_chk;
ALTER TABLE person_bonds ADD CONSTRAINT person_bonds_kind_chk CHECK (kind IN (
    'acquaintance', 'colleague', 'comrade', 'friend', 'relative', 'spouse', 'adversary', 'other'
));

ALTER TABLE person_bond_events DROP CONSTRAINT person_bond_events_kind_chk;
ALTER TABLE person_bond_events ADD CONSTRAINT person_bond_events_kind_chk CHECK (kind IN (
    'acquaintance', 'colleague', 'comrade', 'friend', 'relative', 'spouse', 'adversary', 'other'
));

-- +goose Down
ALTER TABLE person_bond_events DROP CONSTRAINT person_bond_events_kind_chk;
ALTER TABLE person_bond_events ADD CONSTRAINT person_bond_events_kind_chk CHECK (kind IN (
    'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary', 'other'
));

ALTER TABLE person_bonds DROP CONSTRAINT person_bonds_kind_chk;
ALTER TABLE person_bonds ADD CONSTRAINT person_bonds_kind_chk CHECK (kind IN (
    'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary', 'other'
));
