use aes_gcm::aead::{Aead, KeyInit};
use aes_gcm::{Aes256Gcm, Nonce};
use anyhow::{anyhow, Context, Result};
use rand::rngs::OsRng;
use rand::RngCore;

pub struct TokenBox {
    cipher: Aes256Gcm,
}

impl TokenBox {
    pub fn new(hex_key: &str) -> Result<Self> {
        let key = hex::decode(hex_key.trim()).context("token key hex")?;
        if key.len() != 32 {
            return Err(anyhow!("token key must be 32 bytes"));
        }
        Ok(Self {
            cipher: Aes256Gcm::new_from_slice(&key).map_err(|_| anyhow!("invalid token key"))?,
        })
    }

    pub fn seal(&self, plaintext: &str) -> Result<String> {
        let mut nonce = [0_u8; 12];
        OsRng.fill_bytes(&mut nonce);
        let ciphertext = self
            .cipher
            .encrypt(Nonce::from_slice(&nonce), plaintext.as_bytes())
            .map_err(|_| anyhow!("seal token"))?;
        let mut output = Vec::with_capacity(nonce.len() + ciphertext.len());
        output.extend_from_slice(&nonce);
        output.extend_from_slice(&ciphertext);
        Ok(hex::encode(output))
    }

    pub fn open(&self, sealed: &str) -> Result<String> {
        let raw = hex::decode(sealed).context("sealed token")?;
        if raw.len() < 12 {
            return Err(anyhow!("sealed token too short"));
        }
        let plain = self
            .cipher
            .decrypt(Nonce::from_slice(&raw[..12]), &raw[12..])
            .map_err(|_| anyhow!("open token"))?;
        String::from_utf8(plain).context("token is not UTF-8")
    }
}

#[cfg(test)]
mod tests {
    use super::TokenBox;

    #[test]
    fn token_round_trip_and_rejects_wrong_key() {
        let box_key =
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap();
        let sealed = box_key.seal("source credential").unwrap();
        assert_eq!(box_key.open(&sealed).unwrap(), "source credential");
        assert!(TokenBox::new("bad").is_err());
        assert!(
            TokenBox::new("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
                .unwrap()
                .open(&sealed)
                .is_err()
        );
    }
    #[test]
    fn opens_go_ciphertext() {
        let box_key =
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap();
        let sealed = "29c0d07e0b6e31953203f1be0250c035eeb0ab1a5d0aafa4eb228da63e4a519b42d8a7866459c871957a0fcda3";
        assert_eq!(box_key.open(sealed).unwrap(), "source credential");
    }
}
