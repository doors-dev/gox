use std::ptr;
mod formatter;
mod init;

#[repr(C)]
pub struct Buf {
    pub ptr: *mut u8,
    pub len: usize,
    pub cap: usize,
    pub res: i32, 
}

#[unsafe(no_mangle)]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn format(ptr_in: *const u8, len_in: usize) -> Buf {
    if ptr_in.is_null() && len_in != 0 {
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            res: 1,
        };
    }
    let bytes: &[u8] = if len_in == 0 {
        &[]
    } else {
        unsafe { std::slice::from_raw_parts(ptr_in, len_in) }
    };
    let mut output = Vec::new();
    let res = formatter::format(bytes, &mut output);
    if let Err(err) = res {
        let reason = match err {
            topiary_core::FormatterError::Idempotence => 2,
            topiary_core::FormatterError::IdempotenceParsing(_) => 3,
            topiary_core::FormatterError::Internal(_, _) => 4,
            topiary_core::FormatterError::Parsing(_) => 5,
            topiary_core::FormatterError::PatternDoesNotMatch => 6,
            topiary_core::FormatterError::Query(_, _) => 7,
            topiary_core::FormatterError::Io(_) => 8,
        };
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            res: reason,
        };
    }

    if output.is_empty() {
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            res: 0,
        };
    }
    let len = output.len();
    let ptr = output.as_mut_ptr();
    let cap = output.capacity();
    if output.as_slice() == bytes {
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            res: -1,
        };
    }
    std::mem::forget(output);
    Buf {
        ptr,
        len,
        cap,
        res: 0,
    }
}

#[unsafe(no_mangle)]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn free_buf(ptr: *mut u8, len: usize, cap: usize) {
    if ptr.is_null() || cap == 0 {
        return;
    }
    unsafe {
        drop(Vec::from_raw_parts(ptr, len, cap));
    }
}
